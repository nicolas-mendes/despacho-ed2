package main

import (
	"encoding/json"
	"io"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Chamado struct {
	ID     int64
	X, Y   int
	Valor  int
	Prazo  int
	Aberto bool
}

type Ambulancia struct {
	Equipe  int
	ID      int
	X, Y    int
	Ocupada bool
	Valor   int // valor do chamado do paciente a bordo
}

type Equipe struct {
	Num        int
	Bot        *Bot
	Ambs       []*Ambulancia
	Pontos     int
	Entregas   int
	Recolhidos int
}

type Jogo struct {
	C       Cenario
	Etapa   int
	Mapa    *Mapa
	Equipes []*Equipe

	geracao   *rand.Rand // só gera chamados: a sequência não depende dos bots
	desempate *rand.Rand

	porID      map[int64]*Chamado
	usados     map[int64]bool // todos os identificadores já sorteados
	porCelula  map[int][]*Chamado
	expiraEm   [][]*Chamado
	recolhidos []int64 // recolhidos no turno anterior

	replay io.Writer

	// MedirCPU: o estouro é decidido pelo tempo de processador do bot no
	// turno; o relógio vale só como teto, para um bot travado não travar a
	// partida. Sem MedirCPU, o limite é o tempo de relógio.
	MedirCPU bool
}

func novoJogo(c Cenario, etapa int, semente uint64, bots []*Bot) *Jogo {
	j := &Jogo{
		C:         c,
		Etapa:     etapa,
		geracao:   rand.New(rand.NewPCG(semente, 1)),
		desempate: rand.New(rand.NewPCG(semente, 3)),
		porID:     map[int64]*Chamado{},
		usados:    map[int64]bool{},
		porCelula: map[int][]*Chamado{},
		expiraEm:  make([][]*Chamado, c.Turnos+c.PrazoMax+2),
	}
	j.Mapa = gerarMapa(c, etapa, rand.New(rand.NewPCG(semente, 2)))
	for e, b := range bots {
		eq := &Equipe{Num: e, Bot: b}
		for a := 0; a < c.Ambulancias; a++ {
			h := j.Mapa.Hospitais[a%len(j.Mapa.Hospitais)]
			eq.Ambs = append(eq.Ambs, &Ambulancia{Equipe: e, ID: a, X: h[0], Y: h[1]})
		}
		j.Equipes = append(j.Equipes, eq)
	}
	return j
}

// campos escreve inteiros separados por espaço; linha acrescenta a quebra.
func campos(b []byte, valores ...int) []byte {
	for i, v := range valores {
		if i > 0 {
			b = append(b, ' ')
		}
		b = strconv.AppendInt(b, int64(v), 10)
	}
	return b
}

func linha(b []byte, valores ...int) []byte {
	return append(campos(b, valores...), '\n')
}

func (j *Jogo) mensagemInicio(e *Equipe) []byte {
	c, m := j.C, j.Mapa
	b := []byte("INICIO ")
	b = linha(b, c.Largura, c.Altura, c.Turnos, c.LimiteTurnoMs, c.LimitePreparoMs, e.Num, len(j.Equipes))
	b = append(b, "HOSPITAIS "...)
	b = linha(b, len(m.Hospitais))
	for _, h := range m.Hospitais {
		b = linha(b, h[0], h[1])
	}
	b = append(b, "AMBULANCIAS "...)
	b = linha(b, len(e.Ambs))
	for _, a := range e.Ambs {
		b = linha(b, a.ID, a.X, a.Y)
	}
	b = append(b, "BLOQUEIOS "...)
	b = linha(b, len(m.Retangulos))
	for _, r := range m.Retangulos {
		b = linha(b, r[0], r[1], r[2], r[3])
	}
	return append(b, "FIM\n"...)
}

func (j *Jogo) mensagemTurno(t int, novos []*Chamado, e *Equipe) []byte {
	b := []byte("TURNO ")
	b = linha(b, t)
	b = append(b, "CHAMADOS "...)
	b = linha(b, len(novos))
	for _, c := range novos {
		b = strconv.AppendInt(b, c.ID, 10)
		b = append(b, ' ')
		b = linha(b, c.X, c.Y, c.Valor, c.Prazo)
	}
	b = append(b, "RECOLHIDOS "...)
	b = linha(b, len(j.recolhidos))
	for _, id := range j.recolhidos {
		b = strconv.AppendInt(b, id, 10)
		b = append(b, '\n')
	}
	b = append(b, "AMBULANCIAS "...)
	b = linha(b, len(e.Ambs))
	for _, a := range e.Ambs {
		b = campos(b, a.ID, a.X, a.Y)
		if a.Ocupada {
			b = append(b, " OCUPADA\n"...)
		} else {
			b = append(b, " LIVRE\n"...)
		}
	}
	b = append(b, "PLACAR "...)
	b = linha(b, len(j.Equipes))
	for _, eq := range j.Equipes {
		b = linha(b, eq.Num, eq.Pontos)
	}
	return append(b, "FIM\n"...)
}

func (j *Jogo) gerarChamados(t int) []*Chamado {
	c := j.C
	n := c.ChamadosPorTurno
	if v := c.ChamadosPorTurno / 4; v > 0 {
		n += j.geracao.IntN(2*v+1) - v
	}
	novos := make([]*Chamado, 0, n)
	for i := 0; i < n; i++ {
		var id int64
		for {
			id = 1 + j.geracao.Int64N(1<<31-1)
			if !j.usados[id] {
				j.usados[id] = true
				break
			}
		}
		x, y := j.Mapa.sortearChamado(j.geracao, c.FracaoFocos, c.RaioFoco)
		ch := &Chamado{ID: id, X: x, Y: y, Aberto: true}
		if j.geracao.Float64() < c.FracaoGraves {
			ch.Valor = c.ValorGraveMin + j.geracao.IntN(100-c.ValorGraveMin+1)
			ch.Prazo = t + c.PrazoGraveMin + j.geracao.IntN(c.PrazoGraveMax-c.PrazoGraveMin+1)
		} else {
			ch.Valor = 1 + j.geracao.IntN(c.ValorComumMax)
			ch.Prazo = t + c.PrazoMin + j.geracao.IntN(c.PrazoMax-c.PrazoMin+1)
		}
		j.porID[id] = ch
		cel := j.Mapa.indice(x, y)
		j.porCelula[cel] = append(j.porCelula[cel], ch)
		j.expiraEm[ch.Prazo] = append(j.expiraEm[ch.Prazo], ch)
		novos = append(novos, ch)
	}
	return novos
}

func (j *Jogo) fechar(ch *Chamado) {
	ch.Aberto = false
	cel := j.Mapa.indice(ch.X, ch.Y)
	lista := j.porCelula[cel]
	for i, o := range lista {
		if o == ch {
			lista[i] = lista[len(lista)-1]
			lista = lista[:len(lista)-1]
			break
		}
	}
	if len(lista) == 0 {
		delete(j.porCelula, cel)
	} else {
		j.porCelula[cel] = lista
	}
}

// aplicarResposta converte as linhas do bot em movimentos e os aplica.
func (j *Jogo) aplicarResposta(e *Equipe, r *Resposta) {
	if r == nil {
		return
	}
	visto := make([]bool, len(e.Ambs))
	for _, l := range r.Linhas {
		campos := strings.Fields(l)
		if len(campos) == 0 {
			continue
		}
		if len(campos) != 2 {
			e.Bot.Infracoes++
			continue
		}
		id, err := strconv.Atoi(campos[0])
		if err != nil || id < 0 || id >= len(e.Ambs) || visto[id] {
			e.Bot.Infracoes++
			continue
		}
		visto[id] = true
		a := e.Ambs[id]
		nx, ny := a.X, a.Y
		switch campos[1] {
		case "N":
			ny--
		case "S":
			ny++
		case "L":
			nx++
		case "O":
			nx--
		case "P":
		default:
			e.Bot.Infracoes++
			continue
		}
		if !j.Mapa.Livre(nx, ny) {
			e.Bot.Infracoes++
			continue
		}
		a.X, a.Y = nx, ny
	}
}

func (j *Jogo) resolverTurno(t int) {
	// Recolhimento: ambulâncias livres agrupadas por célula, na ordem fixa
	// (equipe, id), para o resultado depender só da semente.
	var celulas []int
	livresEm := map[int][]*Ambulancia{}
	for _, e := range j.Equipes {
		for _, a := range e.Ambs {
			if a.Ocupada {
				continue
			}
			cel := j.Mapa.indice(a.X, a.Y)
			if _, ok := j.porCelula[cel]; !ok {
				continue
			}
			if _, ok := livresEm[cel]; !ok {
				celulas = append(celulas, cel)
			}
			livresEm[cel] = append(livresEm[cel], a)
		}
	}
	var recolhidos []int64
	for _, cel := range celulas {
		ambs := livresEm[cel]
		j.desempate.Shuffle(len(ambs), func(p, q int) { ambs[p], ambs[q] = ambs[q], ambs[p] })
		chs := append([]*Chamado(nil), j.porCelula[cel]...)
		sort.Slice(chs, func(p, q int) bool {
			if chs[p].Valor != chs[q].Valor {
				return chs[p].Valor > chs[q].Valor
			}
			return chs[p].ID < chs[q].ID
		})
		for i := 0; i < len(ambs) && i < len(chs); i++ {
			ch := chs[i]
			a := ambs[i]
			a.Ocupada = true
			a.Valor = ch.Valor
			j.Equipes[a.Equipe].Recolhidos++
			j.fechar(ch)
			recolhidos = append(recolhidos, ch.ID)
		}
	}
	// Entrega.
	for _, e := range j.Equipes {
		for _, a := range e.Ambs {
			if a.Ocupada && j.Mapa.EhHospital[j.Mapa.indice(a.X, a.Y)] {
				a.Ocupada = false
				e.Pontos += a.Valor
				e.Entregas++
				a.Valor = 0
			}
		}
	}
	// Expiração dos chamados com prazo t.
	for _, ch := range j.expiraEm[t] {
		if ch.Aberto {
			j.fechar(ch)
		}
		delete(j.porID, ch.ID)
	}
	j.expiraEm[t] = nil
	for _, id := range recolhidos {
		delete(j.porID, id)
	}
	j.recolhidos = recolhidos
}

type registroTurno struct {
	T          int        `json:"t"`
	Novos      [][5]int64 `json:"novos"`
	Recolhidos []int64    `json:"recolhidos"`
	Ambs       [][5]int   `json:"ambs"` // equipe, id, x, y, ocupada
	Placar     []int      `json:"placar"`
}

func (j *Jogo) gravarInicio() {
	if j.replay == nil {
		return
	}
	json.NewEncoder(j.replay).Encode(map[string]any{
		"cenario": j.C.Nome, "etapa": j.Etapa, "largura": j.C.Largura, "altura": j.C.Altura,
		"turnos": j.C.Turnos, "hospitais": j.Mapa.Hospitais, "bloqueios": j.Mapa.Retangulos,
		"equipes": len(j.Equipes), "bots": j.nomesBots(),
	})
}

func (j *Jogo) nomesBots() []string {
	var nomes []string
	for _, e := range j.Equipes {
		nomes = append(nomes, filepath.Base(e.Bot.Caminho))
	}
	return nomes
}

func (j *Jogo) gravarTurno(t int, novos []*Chamado) {
	if j.replay == nil {
		return
	}
	r := registroTurno{T: t, Recolhidos: j.recolhidos}
	for _, c := range novos {
		r.Novos = append(r.Novos, [5]int64{c.ID, int64(c.X), int64(c.Y), int64(c.Valor), int64(c.Prazo)})
	}
	for _, e := range j.Equipes {
		r.Placar = append(r.Placar, e.Pontos)
		for _, a := range e.Ambs {
			oc := 0
			if a.Ocupada {
				oc = 1
			}
			r.Ambs = append(r.Ambs, [5]int{e.Num, a.ID, a.X, a.Y, oc})
		}
	}
	json.NewEncoder(j.replay).Encode(r)
}

func (j *Jogo) Rodar() {
	limiteTurno := time.Duration(j.C.LimiteTurnoMs) * time.Millisecond
	limitePreparo := time.Duration(j.C.LimitePreparoMs) * time.Millisecond
	tetoTurno := limiteTurno
	if j.MedirCPU {
		tetoTurno = max(10*limiteTurno, 100*time.Millisecond)
	}
	j.gravarInicio()

	for _, e := range j.Equipes {
		r := e.Bot.Enviar(0, j.mensagemInicio(e), limitePreparo)
		if r != nil && (len(r.Linhas) != 1 || strings.TrimSpace(r.Linhas[0]) != "PRONTO") {
			e.Bot.Infracoes++
		}
		e.Bot.MedirMemoria(j.C.MemoriaMB)
	}

	for t := 1; t <= j.C.Turnos; t++ {
		novos := j.gerarChamados(t)
		// Os bots jogam um de cada vez, para não disputarem processador.
		for _, e := range j.Equipes {
			r := e.Bot.Enviar(t, j.mensagemTurno(t, novos, e), tetoTurno)
			if j.MedirCPU && r != nil && r.Gasto > limiteTurno {
				// A resposta gastou mais processador que o limite e é
				// descartada: as ambulâncias ficam paradas. Se é a do próprio
				// turno, conta o estouro. Se é uma resposta atrasada do turno
				// anterior, o estouro já foi contado no turno dela, quando o
				// teto de relógio venceu. Assim cada turno conta no máximo um.
				if r.Seq == t {
					e.Bot.Estouros++
				}
				r = nil
			}
			j.aplicarResposta(e, r)
			e.Bot.MedirMemoria(j.C.MemoriaMB)
		}
		j.resolverTurno(t)
		j.gravarTurno(t, novos)
	}
	for _, e := range j.Equipes {
		e.Bot.Encerrar()
	}
}
