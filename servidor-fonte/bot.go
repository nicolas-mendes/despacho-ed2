package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Resposta é um bloco de linhas do bot terminado por FIM. A resposta 0 é a
// linha PRONTO; a resposta t é a do turno t.
//
// CPU é o tempo de processador acumulado do bot no momento em que a resposta
// chegou; Gasto, o processador consumido desde a chegada da resposta anterior.
// Assim, quando o bot se atrasa, o trabalho que sobra de um turno é cobrado
// da resposta daquele turno, e não da do turno seguinte.
type Resposta struct {
	Seq     int
	Linhas  []string
	Chegada time.Time
	CPU     time.Duration
	Gasto   time.Duration
}

type envio struct {
	seq int
	msg []byte
}

type escrito struct {
	seq int
	em  time.Time
}

// Bot é o processo de uma equipe. A escrita e a leitura correm em
// goroutines próprias, para que um bot travado não trave o servidor.
type Bot struct {
	Caminho string
	cmd     *exec.Cmd

	fila      chan envio
	escritos  chan escrito
	respostas chan Resposta
	fimSaida  chan struct{}

	envios      map[int]time.Time
	ultimaUsada int
	pendente    *Resposta

	Morto          bool
	Motivo         string
	Infracoes      int
	Estouros       int
	EstouroPreparo bool
	TempoPreparo   time.Duration
	Tempos         []time.Duration
	TurnosTempos   []int           // turno de cada valor em Tempos
	TemposCPU      []time.Duration // processador de cada resposta de turno
	cpuUltima      time.Duration   // CPU da última resposta recebida
	MemMaxKB       int
}

// iniciarBot executa o bot. Com nucleo >= 0, o bot roda preso a esse núcleo
// (taskset), para não migrar entre núcleos de desempenho diferente.
func iniciarBot(caminho string, nucleo int) (*Bot, error) {
	b := &Bot{
		Caminho:   caminho,
		fila:      make(chan envio, 4096),
		escritos:  make(chan escrito, 4096),
		respostas: make(chan Resposta, 4096),
		fimSaida:  make(chan struct{}),
		envios:    map[int]time.Time{},
	}
	if nucleo >= 0 {
		b.cmd = exec.Command("taskset", "-c", strconv.Itoa(nucleo), caminho)
	} else {
		b.cmd = exec.Command(caminho)
	}
	b.cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
	b.cmd.Stderr = io.Discard
	entrada, err := b.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	saida, err := b.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := b.cmd.Start(); err != nil {
		return nil, err
	}
	go b.escrever(entrada)
	go b.ler(saida)
	return b, nil
}

func (b *Bot) escrever(entrada io.WriteCloser) {
	w := bufio.NewWriterSize(entrada, 1<<16)
	for e := range b.fila {
		if _, err := w.Write(e.msg); err != nil {
			break
		}
		if err := w.Flush(); err != nil {
			break
		}
		b.escritos <- escrito{e.seq, time.Now()}
	}
	entrada.Close()
	for range b.fila {
	}
}

func (b *Bot) ler(saida io.Reader) {
	defer close(b.fimSaida)
	sc := bufio.NewScanner(saida)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	if !sc.Scan() {
		return
	}
	b.respostas <- Resposta{Seq: 0, Linhas: []string{sc.Text()}, Chegada: time.Now(), CPU: b.TempoCPU()}
	seq := 1
	var linhas []string
	for sc.Scan() {
		l := sc.Text()
		if strings.TrimSpace(l) == "FIM" {
			b.respostas <- Resposta{Seq: seq, Linhas: linhas, Chegada: time.Now(), CPU: b.TempoCPU()}
			seq++
			linhas = nil
			continue
		}
		linhas = append(linhas, l)
	}
}

// Enviar entrega a mensagem do turno seq ao bot e devolve a resposta usada
// neste turno, ou nil se não houver nenhuma a tempo. O tempo do bot começa a
// contar quando a mensagem inteira foi escrita no pipe.
func (b *Bot) Enviar(seq int, msg []byte, limite time.Duration) *Resposta {
	if b.Morto {
		return nil
	}
	b.fila <- envio{seq, msg}
	prazoEscrita := time.NewTimer(limite)
	defer prazoEscrita.Stop()
	for {
		select {
		case e := <-b.escritos:
			b.envios[e.seq] = e.em
			if e.seq != seq {
				continue
			}
			return b.esperar(seq, e.em.Add(limite))
		case <-prazoEscrita.C:
			// O bot não leu a mensagem a tempo.
			b.contarEstouro(seq)
			return b.usarPendente()
		}
	}
}

func (b *Bot) esperar(seq int, prazo time.Time) *Resposta {
	timer := time.NewTimer(time.Until(prazo))
	defer timer.Stop()
	for {
		select {
		case r := <-b.respostas:
			if r := b.receber(r, seq); r != nil {
				return r
			}
		case <-timer.C:
			b.contarEstouro(seq)
			return b.usarPendente()
		case <-b.fimSaida:
		esvaziar:
			for {
				select {
				case r := <-b.respostas:
					if r := b.receber(r, seq); r != nil {
						return r
					}
				default:
					break esvaziar
				}
			}
			b.Morto = true
			b.Motivo = "saída do bot encerrada"
			return b.usarPendente()
		}
	}
}

func (b *Bot) receber(r Resposta, seq int) *Resposta {
	// As respostas chegam em ordem, então a diferença para a anterior é o
	// processador gasto nesta. Se o processo já terminou, a leitura de /proc
	// volta zero, e o gasto fica zero em vez de negativo.
	if r.CPU > b.cpuUltima {
		r.Gasto = r.CPU - b.cpuUltima
		b.cpuUltima = r.CPU
	}
	if r.Seq > 0 {
		b.TemposCPU = append(b.TemposCPU, r.Gasto)
	}
	if t0, ok := b.envios[r.Seq]; ok {
		d := r.Chegada.Sub(t0)
		if r.Seq == 0 {
			b.TempoPreparo = d
		} else {
			b.Tempos = append(b.Tempos, d)
			b.TurnosTempos = append(b.TurnosTempos, r.Seq)
		}
		delete(b.envios, r.Seq)
	}
	if r.Seq == seq {
		b.ultimaUsada = seq
		b.pendente = nil
		return &r
	}
	if r.Seq > b.ultimaUsada {
		b.pendente = &r
	}
	return nil
}

func (b *Bot) usarPendente() *Resposta {
	r := b.pendente
	if r == nil || r.Seq <= b.ultimaUsada || r.Seq == 0 {
		return nil
	}
	b.ultimaUsada = r.Seq
	b.pendente = nil
	return r
}

func (b *Bot) contarEstouro(seq int) {
	if seq == 0 {
		b.EstouroPreparo = true
	} else {
		b.Estouros++
	}
}

// TempoCPU soma o tempo de execução de todas as threads do processo, lido
// de /proc/<pid>/task/*/schedstat (primeiro campo, em nanossegundos).
func (b *Bot) TempoCPU() time.Duration {
	if b.cmd.Process == nil {
		return 0
	}
	tarefas, _ := filepath.Glob("/proc/" + strconv.Itoa(b.cmd.Process.Pid) + "/task/*/schedstat")
	var total int64
	for _, t := range tarefas {
		dados, err := os.ReadFile(t)
		if err != nil {
			continue
		}
		campos := strings.Fields(string(dados))
		if len(campos) > 0 {
			v, _ := strconv.ParseInt(campos[0], 10, 64)
			total += v
		}
	}
	return time.Duration(total)
}

// MedirMemoria lê o VmRSS do processo e encerra o bot acima do limite.
func (b *Bot) MedirMemoria(limiteMB int) {
	if b.Morto || b.cmd.Process == nil {
		return
	}
	dados, err := os.ReadFile("/proc/" + strconv.Itoa(b.cmd.Process.Pid) + "/status")
	if err != nil {
		return
	}
	for _, l := range strings.Split(string(dados), "\n") {
		if strings.HasPrefix(l, "VmRSS:") {
			campos := strings.Fields(l)
			kb, _ := strconv.Atoi(campos[1])
			if kb > b.MemMaxKB {
				b.MemMaxKB = kb
			}
			if kb > limiteMB*1024 {
				b.Morto = true
				b.Motivo = "memória acima do limite"
				b.cmd.Process.Kill()
			}
		}
	}
}

func (b *Bot) Encerrar() {
	close(b.fila)
	feito := make(chan struct{})
	go func() { b.cmd.Wait(); close(feito) }()
	select {
	case <-feito:
	case <-time.After(2 * time.Second):
		b.cmd.Process.Kill()
		<-feito
	}
}
