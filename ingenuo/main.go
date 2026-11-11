package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Chamado struct {
	ID         int
	X, Y       int
	Valor      int
	Prazo      int
	TurnoMirou int //último turno que uma amb mirou este cham, se diferente do atual -> abandonado
}

type Ambulancia struct {
	ID    int
	X, Y  int
	Livre bool
}

type Hospital struct {
	X, Y int
}

// Tabela de dispersão (hash)
type Tabela struct {
	chaves  []int
	valores []int
	ocupada []bool
	n       int
}

const (
	tam_bloco         = 50                                                  // quantidade de celulas no lado do bloco (total_celulas = tam_bloco²)
	chamado_barato    = 14                                                  // valores abaixo desse são ignorados
	distancia_worth   = 3                                                   // quantas vezes a distancia de um novo chamado pode ser maior para compensar a troca para o atual
	valor_worth       = 3                                                   // quantas vezes o valor de um novo chamado deve ser maior para compensar a troca para o atual
	valor_max_chamado = 100                                                 // valor máximo de um chamado
	valor_min_worth   = (valor_max_chamado + valor_worth - 1) / valor_worth // valor máximo de um chamado atual pode ter para poder haver um melhor conforme prioridade

	// descarta chamados que valem menos 1/n do atual se não tiverem mais perto que 1/m do atual
	descarte_fator_val  = 2
	descarte_fator_dist = 6
	// descarte_fator_val  = valor_worth - 1
	// descarte_fator_dist = distancia_worth + 1
)

var (
	leitor   *bufio.Scanner
	escritor *bufio.Writer

	hospitais []Hospital // slice dos Hospitais disponíveis

	alvo_amb []int // alvo_amb[id_amb] guarda o ID do chamado atribuído a ela (-1 se nenhum)

	//cada elemento de grade é um slice de chamados dentro daquele bloco
	grade   [][]Chamado
	blocosX int
	blocosY int

	map_chamados *Tabela // relaciona ID do chamado -> (bloco*1000000 + indiceNoBloco)

	turno_expira [][]int //guarda a lista de IDs que irão expirar em cada turno
	total_turnos int
)

// # Funções Grade

// indiceBloco devolve o indice do Bloco que contém a célula (x,y)
// || organiza e calcula utilizando row-major-flattening
func indiceBloco(x, y int) int {
	return (y/tam_bloco)*blocosX + (x / tam_bloco)
}

// #

// # Funções Tabela de Dispersão

// NovaTabela() instancia uma nova tabela de dispersão (hash map) de tamanho m
func NovaTabela(m int) *Tabela {
	return &Tabela{
		chaves:  make([]int, m),
		valores: make([]int, m),
		ocupada: make([]bool, m),
	}
}

// Tabela.m() retorna o número de chaves inseridas na tabela
func (t *Tabela) m() int {
	return len(t.chaves)
}

// mistura() gera um valor para o hash embaralhando a chave
// com operações de bit
func mistura(k int) int {
	x := uint64(k)
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int(x >> 1)
}

// Tabela.hash() retorna o hash gerado para a chave informada
// embaralha e realiza o mod| |
func (t *Tabela) hash(k int) int {
	k = mistura(k)
	h := k % t.m()
	if h < 0 {
		h += t.m()
	}
	return h
}

// Tabela.distancia() retorna a distancia (saltos) entre dois hash
// percorrendo a tabela de forma circular
func (t *Tabela) distancia(de, ate int) int {
	return (ate - de + t.m()) % t.m()
}

// Tabela.Busca() retorna o se uma chave está armazenada
// na tabela e se sim o seu valor armazenado
func (t *Tabela) Busca(k int) (int, bool) {
	//inicia a busca no hash da chave
	p := t.hash(k)

	//percorre a tabela a partir do hash, tratando colisão
	for t.ocupada[p] {
		//achou
		if t.chaves[p] == k {
			return t.valores[p], true
		}
		//vai p/ prox posição, de forma circular
		p = (p + 1) % t.m()
	}
	return 0, false
}

// Tabela.Insere() insere o par chave-valor na tabela
func (t *Tabela) Insere(k, v int) {
	p := t.hash(k)

	//inicia uma busca a partir do hash, para caso
	//a chave já esteja inserida
	for t.ocupada[p] {

		//se já foi inserida, troca o valor
		if t.chaves[p] == k {
			t.valores[p] = v
			return
		}

		//continua a busca de forma circular
		p = (p + 1) % t.m()
	}

	//inseri na primeira posição vazia o novo par
	t.chaves[p] = k
	t.valores[p] = v
	t.ocupada[p] = true
	t.n++

	//fator de carga respeitada para redimensionar (é tratado na instancia da tabela para este projeto, não é um problema)
	if 10*t.n > 7*t.m() {
		t.redimensiona(proximoPrimo(2 * t.m()))
	}
}

// Tabela.redimensiona() redimensiona a tabela inteira e reinsere todos
// os pares chave-valor
func (t *Tabela) redimensiona(m int) {
	nova := NovaTabela(m)
	for p := 0; p < t.m(); p++ {
		if t.ocupada[p] {
			nova.Insere(t.chaves[p], t.valores[p])
		}
	}
	*t = *nova
}

// Tabela.Remove() remove um par chave-valor da tabela
// tratando colisões
func (t *Tabela) Remove(k int) bool {
	i := t.hash(k)
	for t.ocupada[i] {
		if t.chaves[i] == k {
			t.ocupada[i] = false
			t.n--
			j := (i + 1) % t.m()
			for t.ocupada[j] {
				h := t.hash(t.chaves[j])
				if t.distancia(i, j) <= t.distancia(h, j) {
					t.chaves[i] = t.chaves[j]
					t.valores[i] = t.valores[j]
					t.ocupada[i] = true
					t.ocupada[j] = false
					i = j
				}
				j = (j + 1) % t.m()
			}
			return true
		}
		i = (i + 1) % t.m()
	}
	return false
}

// proximoPrimo() apenas retorna o próximo número primo
// a partir do valor informado
func proximoPrimo(k int) int {
	for {
		primo := k >= 2
		for d := 2; d*d <= k; d++ {
			if k%d == 0 {
				primo = false
				break
			}
		}
		if primo {
			return k
		}
		k++
	}
}

//#

// lerCampos() lê a próxima linha da entrada e a divide em campos.
// Devolve nil quando a entrada termina.
func lerCampos() []string {
	if !leitor.Scan() {
		return nil
	}
	return strings.Fields(leitor.Text())
}

// inteiro() converte um valor para inteiro
func inteiro(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic("número inválido na entrada: " + s)
	}
	return v
}

// abs() retorna o módulo matemático de um valor ||
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// distancia() retorna a distancia de "passos" entre dois pontos da grade
// considerando que a movimentação é apenas em células
func distancia(x1, y1, x2, y2 int) int {
	return abs(x1-x2) + abs(y1-y2)
}

// direcao() devolve o movimento que aproxima (x, y) de (alvoX, alvoY),
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

// lerInicio() lê a mensagem de início e instância
// todas as Estruturas de Dados necessárias
func lerInicio() {
	inicio := lerCampos()
	// estrutura:
	// | INICIO | largura | altura | turnos | limite_turno_ms | limite_preparo_ms | equipe | num_equipes |

	largura := inteiro(inicio[1])
	altura := inteiro(inicio[2])
	total_turnos = inteiro(inicio[3])

	blocosX = (largura + tam_bloco - 1) / tam_bloco

	// 80 + 20 - 1 = 99
	// 99 / 20 = 4
	// | 0-19 | 20-39 | 40-59 | 60-79 |

	blocosY = (altura + tam_bloco - 1) / tam_bloco

	grade = make([][]Chamado, blocosX*blocosY)

	map_chamados = NovaTabela(1000003)
	turno_expira = make([][]int, total_turnos+2)

	// estrutura:
	// | HOSPITAIS | total |
	h := inteiro(lerCampos()[1])
	for range h {
		// estrutura:
		// | X | Y |
		c := lerCampos()
		hospitais = append(hospitais, Hospital{inteiro(c[0]), inteiro(c[1])})
	}

	// estrutura:
	// | AMBULANCIAS | total |
	k := inteiro(lerCampos()[1])

	//instancia o endereçamento direto conforme total de ambulancias
	alvo_amb = make([]int, k)

	//atribui o id de cada uma no vetor
	for i := range k {
		// estrutura:
		// | id | X | Y |
		lerCampos()
		alvo_amb[i] = -1
	}

	// estrutura:
	// | BLOQUEIOS | total |
	b := inteiro(lerCampos()[1])

	//ignorando bloqueios (etapa1)
	for range b {
		// estrutura:
		// | x1 | y1 | x2 | y2 |
		lerCampos()
	}

	// estrutura:
	// | FIM |
	lerCampos()
}

// removerPorID() localiza o chamado pela Tabela Hash, faz a troca com o
// último elemento do slice e atualiza o índice do elemento movido
func removerPorID(id int) {
	cod_cham, existe := map_chamados.Busca(id)

	if !existe {
		return
	}

	//Ex:
	// 512.000.012 / 1.000.000 = 512
	// 512.000.012 % 1.000.000 = 12
	bloco := cod_cham / 1000000
	id_slice_cham := cod_cham % 1000000

	//pega o ultimo chamado do slice
	id_slice_ultimo := len(grade[bloco]) - 1

	//se o chamado a ser removido ñ é o ultimo
	if id_slice_cham != id_slice_ultimo {
		//reinsere o ultimo chamado na posição do q será removido
		ultimoChamado := grade[bloco][id_slice_ultimo]
		grade[bloco][id_slice_cham] = ultimoChamado
		map_chamados.Insere(ultimoChamado.ID, bloco*1000000+id_slice_cham)
	}

	//remove o ultimo chamado do slice
	grade[bloco] = grade[bloco][:id_slice_ultimo]
	map_chamados.Remove(id)
}

// adicionarChamado() adiciona os chamados em map_chamados e atualiza a tabela de expiração
func adicionarChamado(ch Chamado) {
	//Ignora chamados "baratos"
	if ch.Valor < chamado_barato {
		return
	}

	bloco_alvo := indiceBloco(ch.X, ch.Y)
	indice_slice_cham := len(grade[bloco_alvo])

	//insere no slice de chamados do bloco o chamado novo
	grade[bloco_alvo] = append(grade[bloco_alvo], ch)

	//atualiza o hash_map de chamados
	map_chamados.Insere(ch.ID, bloco_alvo*1000000+indice_slice_cham)

	//atualiza a tabela de expiração ignorando chamados que não serão removidos (tempo além do limite da sessão)
	if ch.Prazo <= total_turnos {
		turno_expira[ch.Prazo] = append(turno_expira[ch.Prazo], ch.ID)
	}
}

// removerExpirados() limpa todos os chamados que já expiraram
func removerExpirados(turno int) {
	ultimo_turno := turno - 1

	if ultimo_turno >= 1 && ultimo_turno <= total_turnos {
		//remove todos os chamados que expiraram
		for _, id := range turno_expira[ultimo_turno] {
			removerPorID(id)
		}

		turno_expira[ultimo_turno] = nil
	}
}

// jogarTurno() faz mágica
func jogarTurno(turno int, ambulancias []Ambulancia) {

	//percorre cada ambulância
	for _, a := range ambulancias {

		mov := "P"

		if a.Livre {

			//valores do melhor chamado (alvo) conforme prioridades
			bloco_melhor := -1
			id_melhor := -1
			dist_melhor := 0
			val_melhor := 0

			// 1. Se já tinha um alvo do turno anterior
			// ele é o alvo inicial
			if alvo_amb[a.ID] != -1 {
				cod, achou := map_chamados.Busca(alvo_amb[a.ID])
				if achou {
					bloco_alvo := cod / 1000000
					indice_slice_cham := cod % 1000000

					alvo := grade[bloco_alvo][indice_slice_cham]

					d := distancia(a.X, a.Y, alvo.X, alvo.Y)

					//ainda é alcançável a tempo e não está sendo buscando por mais alguém
					if turno+d <= alvo.Prazo && alvo.TurnoMirou != turno {
						bloco_melhor = bloco_alvo
						id_melhor = indice_slice_cham
						dist_melhor = d
						val_melhor = alvo.Valor
					}
				}
			}

			// 2. Percorre os blocos vizinhos (de forma circular, em anéis) para validar
			// se não existe um chamado melhor conforme prioridades
			x_bloco_amb := a.X / tam_bloco
			y_bloco_amb := a.Y / tam_bloco

			//varre os blocos em anéis até o limite
			for raio := 0; ; raio++ {

				// já tem um alvo e esta buscando além do bloco atual
				if bloco_melhor != -1 && raio > 1 {

					//distância mínima que qualquer chamado terá estando dentro deste anel
					dist_min_cham_no_anel := (raio - 1) * tam_bloco
					distancia_limite_busca := dist_melhor * distancia_worth

					//impossível ter chamado mais caro que o atual, logo só busca mais próximos
					if val_melhor >= valor_min_worth {
						distancia_limite_busca = dist_melhor
					}

					//nenhum chamado (neste anel ou nos mais afastados) poderá satisfazer as prioridades
					if dist_min_cham_no_anel > distancia_limite_busca {
						break
					}
				}

				//limites do anel de busca
				x_min_bloco, x_max_bloco := x_bloco_amb-raio, x_bloco_amb+raio
				y_min_bloco, y_max_bloco := y_bloco_amb-raio, y_bloco_amb+raio

				//valida se ainda existe alguma parte do anel de busca dentro dos limites do mapa
				if x_min_bloco >= 0 || x_max_bloco < blocosX || y_min_bloco >= 0 || y_max_bloco < blocosY {
					//limpa os limites para manter a busca

					if x_min_bloco < 0 {
						x_min_bloco = 0
					}
					if x_max_bloco >= blocosX {
						x_max_bloco = blocosX - 1
					}
					if y_min_bloco < 0 {
						y_min_bloco = 0
					}
					if y_max_bloco >= blocosY {
						y_max_bloco = blocosY - 1
					}

				} else { //limpou o mapa inteiro
					break
				}

				for by := y_min_bloco; by <= y_max_bloco; by++ {
					for bx := x_min_bloco; bx <= x_max_bloco; bx++ {
						//pula blocos que já foram percorridos (internos ao anel de busca)
						if bx != x_bloco_amb-raio && bx != x_bloco_amb+raio && by != y_bloco_amb-raio && by != y_bloco_amb+raio {
							continue
						}

						//indíce unidimensional do bloco
						indice_bloco := by*blocosX + bx

						//percorre cada chamado do bloco
						for idx_chamado, chamado := range grade[indice_bloco] {
							// Ignora se:
							// outra ambulância já está buscando neste turno
							// é o próprio alvo
							if chamado.TurnoMirou == turno || (indice_bloco == bloco_melhor && idx_chamado == id_melhor) {
								continue
							}

							d := distancia(a.X, a.Y, chamado.X, chamado.Y)

							//redundância para chamados inviáveis
							if turno+d > chamado.Prazo || chamado.Valor < chamado_barato {
								continue
							}

							// Se já tem um alvo, descarta chamados mais baratos que não estão muito pertos
							if bloco_melhor != -1 &&
								chamado.Valor*descarte_fator_val < val_melhor &&
								d*descarte_fator_dist >= dist_melhor {
								continue
							}

							//lógica de comparação para achar o melhor chamado

							//primeiro encontrado
							casoInicial := bloco_melhor == -1

							// se der mais pontos e estiver no limite de busca, troca o chamado
							casoVantajoso := chamado.Valor > val_melhor*valor_worth && d <= dist_melhor*distancia_worth

							// se não for o que mais dá pontos, mas está mais perto do que o atual, troca o chamado
							casoProximo := chamado.Valor <= val_melhor*valor_worth && d < dist_melhor

							// se tiver na mesma distância, troca para o que da mais pontos
							casoMesmaDist := d == dist_melhor && chamado.Valor > val_melhor

							if casoInicial || casoVantajoso || casoProximo || casoMesmaDist {
								bloco_melhor = indice_bloco
								id_melhor = idx_chamado
								dist_melhor = d
								val_melhor = chamado.Valor
							}
						}
					}
				}
			}

			// 3. Marca o melhor chamado para este turno e percorre
			if bloco_melhor != -1 {
				grade[bloco_melhor][id_melhor].TurnoMirou = turno

				alvo_amb[a.ID] = grade[bloco_melhor][id_melhor].ID

				alvo := grade[bloco_melhor][id_melhor]

				mov = direcao(a.X, a.Y, alvo.X, alvo.Y)
			} else {
				alvo_amb[a.ID] = -1
			}
		} else { // volta para Hospital

			//limpa alvo antigo (já pegou o paciente)
			alvo_amb[a.ID] = -1
			melhor := 0

			//busca de forma direta o hospital mais próximo
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
		c := lerCampos()

		if c == nil {
			// fim da partida
			return
		}

		// estrutura:
		// | TURNO | id |
		turno := inteiro(c[1])

		// estrutura:
		// | CHAMADOS | total |
		n := inteiro(lerCampos()[1])

		for range n {
			// estrutura:
			// | ID | x | y | valor | prazo |
			c := lerCampos()

			adicionarChamado(Chamado{
				ID:    inteiro(c[0]),
				X:     inteiro(c[1]),
				Y:     inteiro(c[2]),
				Valor: inteiro(c[3]),
				Prazo: inteiro(c[4]),
			})

		}

		// estrutura:
		// | RECOLHIDOS | total |
		r := inteiro(lerCampos()[1])

		for range r {
			// estrutura:
			// | ID |
			removerPorID(inteiro(lerCampos()[0]))
		}

		removerExpirados(turno)

		// estrutura:
		// | AMBULANCIAS | total |
		k := inteiro(lerCampos()[1])
		ambulancias := make([]Ambulancia, k)

		for i := range k {
			// estrutura:
			// | ID | x | y | [LIVRE,OCUPADA] |
			c := lerCampos()

			ambulancias[i] = Ambulancia{inteiro(c[0]), inteiro(c[1]), inteiro(c[2]), c[3] == "LIVRE"}
		}

		// estrutura:
		// | POSIÇÃO | ID |
		e := inteiro(lerCampos()[1])

		for range e {
			lerCampos()
		}

		// estrutura:
		// | FIM |
		lerCampos()

		jogarTurno(turno, ambulancias)
	}
}
