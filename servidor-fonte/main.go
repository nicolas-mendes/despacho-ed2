// Servidor do jogo Central de Despacho (DS143, trabalho prático 2026/2).
//
// Uso:
//
//	servidor [opções] bot1 bot2 ...
//
// Cada bot é um executável que fala o protocolo pela entrada e pela saída
// padrão. A equipe 0 é o primeiro bot da lista.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ResumoEquipe struct {
	Equipe         int     `json:"equipe"`
	Bot            string  `json:"bot"`
	Pontos         int     `json:"pontos"`
	Recolhidos     int     `json:"recolhidos"`
	Entregas       int     `json:"entregas"`
	Infracoes      int     `json:"infracoes"`
	Estouros       int     `json:"estouros"`
	EstouroPreparo bool    `json:"estouro_preparo"`
	PreparoMs      float64 `json:"preparo_ms"`
	MediaMs        float64 `json:"media_ms"`
	P50Ms          float64 `json:"p50_ms"`
	P99Ms          float64 `json:"p99_ms"`
	MaxMs          float64 `json:"max_ms"`
	CPUMediaMs     float64 `json:"cpu_media_ms"`
	CPUP99Ms       float64 `json:"cpu_p99_ms"`
	CPUMaxMs       float64 `json:"cpu_max_ms"`
	MemMaxMB       float64 `json:"mem_max_mb"`
	Morto          bool    `json:"morto"`
	Motivo         string  `json:"motivo,omitempty"`
}

type Resumo struct {
	Cenario  string         `json:"cenario"`
	Etapa    int            `json:"etapa"`
	Semente  uint64         `json:"semente"`
	Turnos   int            `json:"turnos"`
	Chamados int            `json:"chamados"`
	Duracao  float64        `json:"duracao_s"`
	Equipes  []ResumoEquipe `json:"equipes"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func estatisticas(tempos []time.Duration) (media, p50, p99, maximo float64) {
	if len(tempos) == 0 {
		return
	}
	t := append([]time.Duration(nil), tempos...)
	sort.Slice(t, func(i, j int) bool { return t[i] < t[j] })
	var soma time.Duration
	for _, v := range t {
		soma += v
	}
	media = ms(soma / time.Duration(len(t)))
	p50 = ms(t[len(t)/2])
	p99 = ms(t[min(len(t)-1, len(t)*99/100)])
	maximo = ms(t[len(t)-1])
	return
}

func main() {
	nome := flag.String("cenario", "pequeno", "pequeno, medio ou estresse")
	etapa := flag.Int("etapa", 1, "1 (grade aberta) ou 2 (com bloqueios)")
	semente := flag.Uint64("semente", 1, "semente da partida")
	replay := flag.String("replay", "", "arquivo de replay (JSON, um objeto por linha)")
	saidaJSON := flag.String("json", "", "grava o resumo da partida em JSON")
	medida := flag.String("medida", "cpu", "cpu (tempo de processador do bot) ou relogio")
	nucleos := flag.String("nucleos", "", "núcleos para prender os bots, um por bot, separados por vírgula (ex.: 2,3)")
	saidaTempos := flag.String("tempos", "", "grava o tempo de cada turno em CSV (equipe,turno,ms)")
	// Ajustes para calibração.
	limite := flag.Int("limite", 0, "limite por turno em ms (0 mantém o do cenário)")
	turnos := flag.Int("turnos", 0, "número de turnos (0 mantém o do cenário)")
	chamados := flag.Int("chamados", 0, "chamados novos por turno (0 mantém o do cenário)")
	prazoMax := flag.Int("prazomax", 0, "prazo máximo (0 mantém o do cenário)")
	ambs := flag.Int("ambulancias", 0, "ambulâncias por equipe (0 mantém o do cenário)")
	flag.Parse()

	c, ok := cenarios[*nome]
	if !ok {
		fmt.Fprintln(os.Stderr, "cenário desconhecido:", *nome)
		os.Exit(2)
	}
	if *etapa != 1 && *etapa != 2 {
		fmt.Fprintln(os.Stderr, "etapa deve ser 1 ou 2")
		os.Exit(2)
	}
	if *limite > 0 {
		c.LimiteTurnoMs = *limite
	}
	if *turnos > 0 {
		c.Turnos = *turnos
	}
	if *chamados > 0 {
		c.ChamadosPorTurno = *chamados
	}
	if *prazoMax > 0 {
		c.PrazoMax = *prazoMax
	}
	if *ambs > 0 {
		c.Ambulancias = *ambs
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "uso: servidor [opções] bot1 bot2 ...")
		os.Exit(2)
	}

	if *medida != "cpu" && *medida != "relogio" {
		fmt.Fprintln(os.Stderr, "medida deve ser cpu ou relogio")
		os.Exit(2)
	}
	var listaNucleos []int
	if *nucleos != "" {
		for _, n := range strings.Split(*nucleos, ",") {
			v, err := strconv.Atoi(strings.TrimSpace(n))
			if err != nil {
				fmt.Fprintln(os.Stderr, "núcleo inválido:", n)
				os.Exit(2)
			}
			listaNucleos = append(listaNucleos, v)
		}
		if len(listaNucleos) != flag.NArg() {
			fmt.Fprintln(os.Stderr, "informe um núcleo por bot")
			os.Exit(2)
		}
	}

	var bots []*Bot
	for i, caminho := range flag.Args() {
		nucleo := -1
		if listaNucleos != nil {
			nucleo = listaNucleos[i]
		}
		b, err := iniciarBot(caminho, nucleo)
		if err != nil {
			fmt.Fprintln(os.Stderr, "não consegui iniciar", caminho+":", err)
			os.Exit(1)
		}
		bots = append(bots, b)
	}

	j := novoJogo(c, *etapa, *semente, bots)
	j.MedirCPU = *medida == "cpu"
	if j.MedirCPU && runtime.GOOS != "linux" {
		// Fora do Linux não há /proc: o limite passa a valer em tempo de
		// relógio. Serve para o aluno testar; a correção oficial roda em Linux.
		fmt.Fprintln(os.Stderr, "aviso: sem /proc neste sistema; medindo o limite em tempo de relógio")
		j.MedirCPU = false
		*medida = "relogio"
	}
	if *replay != "" {
		f, err := os.Create(*replay)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		w := bufio.NewWriterSize(f, 1<<20)
		defer func() { w.Flush(); f.Close() }()
		j.replay = w
	}

	inicio := time.Now()
	j.Rodar()

	r := Resumo{Cenario: c.Nome, Etapa: *etapa, Semente: *semente, Turnos: c.Turnos,
		Chamados: len(j.usados), Duracao: time.Since(inicio).Seconds()}
	for _, e := range j.Equipes {
		b := e.Bot
		media, p50, p99, maximo := estatisticas(b.Tempos)
		cMedia, _, cP99, cMax := estatisticas(b.TemposCPU)
		r.Equipes = append(r.Equipes, ResumoEquipe{
			Equipe: e.Num, Bot: b.Caminho, Pontos: e.Pontos, Recolhidos: e.Recolhidos,
			Entregas: e.Entregas, Infracoes: b.Infracoes, Estouros: b.Estouros,
			EstouroPreparo: b.EstouroPreparo, PreparoMs: ms(b.TempoPreparo),
			MediaMs: media, P50Ms: p50, P99Ms: p99, MaxMs: maximo,
			CPUMediaMs: cMedia, CPUP99Ms: cP99, CPUMaxMs: cMax,
			MemMaxMB: float64(b.MemMaxKB) / 1024, Morto: b.Morto, Motivo: b.Motivo,
		})
	}

	fmt.Printf("Cenário %s, etapa %d, semente %d: %d turnos, %d chamados, %.1f s, limite %d ms (%s)\n",
		r.Cenario, r.Etapa, r.Semente, r.Turnos, r.Chamados, r.Duracao, c.LimiteTurnoMs, *medida)
	for _, e := range r.Equipes {
		fmt.Printf("Equipe %d (%s): %d pontos, %d recolhidos, %d entregas, %d infrações, %d estouros",
			e.Equipe, e.Bot, e.Pontos, e.Recolhidos, e.Entregas, e.Infracoes, e.Estouros)
		if e.EstouroPreparo {
			fmt.Print(", estourou o preparo")
		}
		fmt.Printf("\n  preparo %.1f ms; turno (relógio): média %.2f ms, p50 %.2f ms, p99 %.2f ms, máx %.2f ms; memória %.0f MB\n",
			e.PreparoMs, e.MediaMs, e.P50Ms, e.P99Ms, e.MaxMs, e.MemMaxMB)
		fmt.Printf("  turno (processador): média %.2f ms, p99 %.2f ms, máx %.2f ms\n", e.CPUMediaMs, e.CPUP99Ms, e.CPUMaxMs)
		if e.Morto {
			fmt.Printf("  bot encerrado: %s\n", e.Motivo)
		}
	}
	if *saidaTempos != "" {
		var b []byte
		b = append(b, "equipe,turno,ms\n"...)
		for _, e := range j.Equipes {
			for i, d := range e.Bot.Tempos {
				b = fmt.Appendf(b, "%d,%d,%.3f\n", e.Num, e.Bot.TurnosTempos[i], ms(d))
			}
		}
		os.WriteFile(*saidaTempos, b, 0o644)
	}
	if *saidaJSON != "" {
		dados, _ := json.MarshalIndent(r, "", "  ")
		os.WriteFile(*saidaJSON, dados, 0o644)
	}
}
