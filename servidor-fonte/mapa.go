package main

import (
	"math"
	"math/rand/v2"
)

// Mapa guarda a grade, os bloqueios e as células onde podem nascer chamados.
type Mapa struct {
	Largura, Altura int
	Bloqueado       []bool   // índice y*Largura + x
	Alcancavel      []bool   // célula livre ligada à componente dos hospitais
	Retangulos      [][4]int // bloqueios como enviados ao bot: x1 y1 x2 y2
	Hospitais       [][2]int
	EhHospital      []bool
	Focos           [][2]int
}

func (m *Mapa) indice(x, y int) int { return y*m.Largura + x }

func (m *Mapa) dentro(x, y int) bool {
	return x >= 0 && x < m.Largura && y >= 0 && y < m.Altura
}

// Livre informa se uma ambulância pode ocupar a célula.
func (m *Mapa) Livre(x, y int) bool {
	return m.dentro(x, y) && !m.Bloqueado[m.indice(x, y)]
}

// valida informa se um chamado pode nascer na célula.
func (m *Mapa) valida(x, y int) bool {
	if !m.dentro(x, y) {
		return false
	}
	i := m.indice(x, y)
	return m.Alcancavel[i] && !m.EhHospital[i]
}

// intervalos divide [0, total) em quarteirões separados por ruas,
// começando e terminando por rua.
func intervalos(total, qmin, qmax, rua int, rng *rand.Rand) [][2]int {
	var r [][2]int
	pos := rua
	for pos < total-rua {
		lado := qmin + rng.IntN(qmax-qmin+1)
		fim := pos + lado - 1
		if fim > total-rua-1 {
			fim = total - rua - 1
		}
		if fim >= pos {
			r = append(r, [2]int{pos, fim})
		}
		pos = fim + 1 + rua
	}
	return r
}

func gerarMapa(c Cenario, etapa int, rng *rand.Rand) *Mapa {
	m := &Mapa{Largura: c.Largura, Altura: c.Altura}
	n := c.Largura * c.Altura
	m.Bloqueado = make([]bool, n)
	m.Alcancavel = make([]bool, n)
	m.EhHospital = make([]bool, n)

	if etapa == 2 {
		colunas := intervalos(c.Largura, c.QuarteiraoMin, c.QuarteiraoMax, c.Rua, rng)
		linhas := intervalos(c.Altura, c.QuarteiraoMin, c.QuarteiraoMax, c.Rua, rng)
		for _, l := range linhas {
			for _, col := range colunas {
				m.Retangulos = append(m.Retangulos, [4]int{col[0], l[0], col[1], l[1]})
			}
		}
		// Trechos de rua fechados: horizontais (entre duas linhas de
		// quarteirões) e verticais (entre duas colunas).
		for j := 0; j+1 < len(linhas); j++ {
			for _, col := range colunas {
				if rng.Float64() < c.FracaoFechadas {
					m.Retangulos = append(m.Retangulos, [4]int{col[0], linhas[j][1] + 1, col[1], linhas[j+1][0] - 1})
				}
			}
		}
		for i := 0; i+1 < len(colunas); i++ {
			for _, l := range linhas {
				if rng.Float64() < c.FracaoFechadas {
					m.Retangulos = append(m.Retangulos, [4]int{colunas[i][1] + 1, l[0], colunas[i+1][0] - 1, l[1]})
				}
			}
		}
		for _, r := range m.Retangulos {
			for y := r[1]; y <= r[3]; y++ {
				for x := r[0]; x <= r[2]; x++ {
					m.Bloqueado[m.indice(x, y)] = true
				}
			}
		}
	}

	// Componente alcançável a partir do canto (0, 0), que é sempre rua.
	fila := []int{0}
	m.Alcancavel[0] = true
	for len(fila) > 0 {
		i := fila[0]
		fila = fila[1:]
		x, y := i%m.Largura, i/m.Largura
		for _, d := range [4][2]int{{0, -1}, {0, 1}, {1, 0}, {-1, 0}} {
			nx, ny := x+d[0], y+d[1]
			if m.Livre(nx, ny) && !m.Alcancavel[m.indice(nx, ny)] {
				m.Alcancavel[m.indice(nx, ny)] = true
				fila = append(fila, m.indice(nx, ny))
			}
		}
	}

	// Hospitais em grade com sorteio dentro de cada faixa, para que fiquem
	// espalhados pelo mapa.
	cols := int(math.Ceil(math.Sqrt(float64(c.Hospitais))))
	lins := (c.Hospitais + cols - 1) / cols
	for h := 0; h < c.Hospitais; h++ {
		cx, cy := h%cols, h/cols
		lx, ly := c.Largura/cols, c.Altura/lins
		x := cx*lx + lx/4 + rng.IntN(lx/2+1)
		y := cy*ly + ly/4 + rng.IntN(ly/2+1)
		x, y = m.maisProximaAlcancavel(x, y)
		m.Hospitais = append(m.Hospitais, [2]int{x, y})
		m.EhHospital[m.indice(x, y)] = true
	}

	for f := 0; f < c.Focos; f++ {
		x, y := m.sortearUniforme(rng)
		m.Focos = append(m.Focos, [2]int{x, y})
	}
	return m
}

// maisProximaAlcancavel procura, em anéis crescentes, a célula alcançável
// e ainda sem hospital mais próxima de (x, y).
func (m *Mapa) maisProximaAlcancavel(x, y int) (int, int) {
	for r := 0; r < m.Largura+m.Altura; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				nx, ny := x+dx, y+dy
				if m.dentro(nx, ny) && m.Alcancavel[m.indice(nx, ny)] && !m.EhHospital[m.indice(nx, ny)] {
					return nx, ny
				}
			}
		}
	}
	panic("mapa sem célula alcançável")
}

func (m *Mapa) sortearUniforme(rng *rand.Rand) (int, int) {
	for {
		x, y := rng.IntN(m.Largura), rng.IntN(m.Altura)
		if m.valida(x, y) {
			return x, y
		}
	}
}

// sortearChamado devolve a posição de um chamado novo: perto de um foco
// com probabilidade fracao, ou uniforme no mapa.
func (m *Mapa) sortearChamado(rng *rand.Rand, fracao, raio float64) (int, int) {
	if len(m.Focos) > 0 && rng.Float64() < fracao {
		f := m.Focos[rng.IntN(len(m.Focos))]
		for tentativa := 0; tentativa < 20; tentativa++ {
			x := f[0] + int(math.Round(rng.NormFloat64()*raio))
			y := f[1] + int(math.Round(rng.NormFloat64()*raio))
			if m.valida(x, y) {
				return x, y
			}
		}
	}
	return m.sortearUniforme(rng)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
