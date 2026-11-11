// Bot ingênuo: ponto de partida do trabalho prático.
//
// Este bot implementa o protocolo inteiro e joga de forma simples:
//
//   - guarda os chamados abertos num slice;
//   - a cada turno, cada ambulância livre procura, por varredura linear,
//     o chamado mais próximo e anda na direção dele;
//   - cada ambulância ocupada anda na direção do hospital mais próximo.
//
// Ele não verifica se o chamado ainda pode ser alcançado dentro do prazo,
// não impede que duas ambulâncias sigam para o mesmo chamado e ignora os
// bloqueios da etapa 2. No cenário de estresse, a varredura linear estoura
// o limite de tempo por turno.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Chamado struct {
	ID    int
	X, Y  int
	Valor int
	Prazo int
}

type Ambulancia struct {
	ID    int
	X, Y  int
	Livre bool
}

type Hospital struct {
	X, Y int
}

var (
	leitor   *bufio.Scanner
	escritor *bufio.Writer

	hospitais []Hospital
	chamados  []Chamado
)

// lerCampos lê a próxima linha da entrada e a divide em campos.
// Devolve nil quando a entrada termina.
func lerCampos() []string {
	if !leitor.Scan() {
		return nil
	}
	return strings.Fields(leitor.Text())
}

func inteiro(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic("número inválido na entrada: " + s)
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func distancia(x1, y1, x2, y2 int) int {
	return abs(x1-x2) + abs(y1-y2)
}

// direcao devolve o movimento que aproxima (x, y) de (alvoX, alvoY),
// andando primeiro na horizontal.
func direcao(x, y, alvoX, alvoY int) string {
	switch {
	case x < alvoX:
		return "L"
	case x > alvoX:
		return "O"
	case y < alvoY:
		return "S"
	case y > alvoY:
		return "N"
	}
	return "P"
}

// lerInicio lê a mensagem de início. Este bot só usa os hospitais; os
// bloqueios são lidos e descartados.
func lerInicio() {
	lerCampos() // INICIO largura altura turnos limite_turno limite_preparo equipe num_equipes
	h := inteiro(lerCampos()[1])
	for i := 0; i < h; i++ {
		c := lerCampos()
		hospitais = append(hospitais, Hospital{inteiro(c[0]), inteiro(c[1])})
	}
	k := inteiro(lerCampos()[1])
	for i := 0; i < k; i++ {
		lerCampos() // id x y
	}
	b := inteiro(lerCampos()[1])
	for i := 0; i < b; i++ {
		lerCampos() // x1 y1 x2 y2
	}
	lerCampos() // FIM
}

// removerPorID procura o chamado no slice e o remove, trocando-o pelo
// último elemento. Custo: proporcional ao número de chamados abertos.
func removerPorID(id int) {
	for i := range chamados {
		if chamados[i].ID == id {
			chamados[i] = chamados[len(chamados)-1]
			chamados = chamados[:len(chamados)-1]
			return
		}
	}
}

// removerExpirados mantém só os chamados com prazo maior ou igual ao turno.
func removerExpirados(turno int) {
	vivos := chamados[:0]
	for _, c := range chamados {
		if c.Prazo >= turno {
			vivos = append(vivos, c)
		}
	}
	chamados = vivos
}

func jogarTurno(turno int, ambulancias []Ambulancia) {
	for _, a := range ambulancias {
		mov := "P"
		if a.Livre {
			melhor := -1
			for i, c := range chamados {
				if melhor == -1 || distancia(a.X, a.Y, c.X, c.Y) < distancia(a.X, a.Y, chamados[melhor].X, chamados[melhor].Y) {
					melhor = i
				}
			}
			if melhor != -1 {
				mov = direcao(a.X, a.Y, chamados[melhor].X, chamados[melhor].Y)
			}
		} else {
			melhor := 0
			for i, h := range hospitais {
				if distancia(a.X, a.Y, h.X, h.Y) < distancia(a.X, a.Y, hospitais[melhor].X, hospitais[melhor].Y) {
					melhor = i
				}
			}
			mov = direcao(a.X, a.Y, hospitais[melhor].X, hospitais[melhor].Y)
		}
		fmt.Fprintln(escritor, a.ID, mov)
	}
	fmt.Fprintln(escritor, "FIM")
	escritor.Flush()
}

func main() {
	leitor = bufio.NewScanner(os.Stdin)
	leitor.Buffer(make([]byte, 1<<16), 1<<20)
	escritor = bufio.NewWriter(os.Stdout)

	lerInicio()
	fmt.Fprintln(escritor, "PRONTO")
	escritor.Flush()

	for {
		c := lerCampos() // TURNO t
		if c == nil {
			return // fim da partida
		}
		turno := inteiro(c[1])

		n := inteiro(lerCampos()[1]) // CHAMADOS n
		for i := 0; i < n; i++ {
			c := lerCampos()
			chamados = append(chamados, Chamado{inteiro(c[0]), inteiro(c[1]), inteiro(c[2]), inteiro(c[3]), inteiro(c[4])})
		}

		r := inteiro(lerCampos()[1]) // RECOLHIDOS r
		for i := 0; i < r; i++ {
			removerPorID(inteiro(lerCampos()[0]))
		}
		removerExpirados(turno)

		k := inteiro(lerCampos()[1]) // AMBULANCIAS k
		ambulancias := make([]Ambulancia, k)
		for i := 0; i < k; i++ {
			c := lerCampos()
			ambulancias[i] = Ambulancia{inteiro(c[0]), inteiro(c[1]), inteiro(c[2]), c[3] == "LIVRE"}
		}

		e := inteiro(lerCampos()[1]) // PLACAR e
		for i := 0; i < e; i++ {
			lerCampos()
		}
		lerCampos() // FIM

		jogarTurno(turno, ambulancias)
	}
}
