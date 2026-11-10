package main

// Cenario reúne os parâmetros de uma partida. Os valores são os da
// especificação; o servidor aceita ajustes por linha de comando para a
// calibração.
type Cenario struct {
	Nome             string
	Largura, Altura  int
	Ambulancias      int // por equipe
	Hospitais        int
	ChamadosPorTurno int // média; o número real varia 25% para cima ou para baixo
	PrazoMin         int // prazo = turno de criação + valor sorteado entre PrazoMin e PrazoMax
	PrazoMax         int
	Turnos           int
	LimiteTurnoMs    int
	LimitePreparoMs  int
	MemoriaMB        int

	// Chamados graves: fração com valor alto e prazo curto. Os demais têm
	// valor entre 1 e ValorComumMax.
	FracaoGraves  float64
	ValorGraveMin int
	ValorComumMax int
	PrazoGraveMin int
	PrazoGraveMax int

	// Focos: parte dos chamados nasce concentrada em torno de pontos fixos.
	Focos       int
	FracaoFocos float64
	RaioFoco    float64

	// Etapa 2: quarteirões com lado sorteado entre QuarteiraoMin e
	// QuarteiraoMax, separados por ruas de largura Rua. Cada trecho de rua
	// entre dois quarteirões fecha com probabilidade FracaoFechadas.
	QuarteiraoMin  int
	QuarteiraoMax  int
	Rua            int
	FracaoFechadas float64
}

var cenarios = map[string]Cenario{
	"pequeno": {
		Nome: "pequeno", Largura: 200, Altura: 200, Ambulancias: 5, Hospitais: 4,
		ChamadosPorTurno: 2, PrazoMin: 50, PrazoMax: 200, Turnos: 500,
		LimiteTurnoMs: 100, LimitePreparoMs: 2000, MemoriaMB: 512,
		FracaoGraves: 0.05, ValorGraveMin: 60, ValorComumMax: 20, PrazoGraveMin: 20, PrazoGraveMax: 80,
		Focos: 2, FracaoFocos: 0.3, RaioFoco: 10,
		QuarteiraoMin: 6, QuarteiraoMax: 14, Rua: 2, FracaoFechadas: 0.15,
	},
	"medio": {
		Nome: "medio", Largura: 1000, Altura: 1000, Ambulancias: 20, Hospitais: 10,
		ChamadosPorTurno: 40, PrazoMin: 100, PrazoMax: 600, Turnos: 1000,
		LimiteTurnoMs: 100, LimitePreparoMs: 2000, MemoriaMB: 512,
		FracaoGraves: 0.05, ValorGraveMin: 60, ValorComumMax: 20, PrazoGraveMin: 30, PrazoGraveMax: 150,
		Focos: 5, FracaoFocos: 0.3, RaioFoco: 30,
		QuarteiraoMin: 6, QuarteiraoMax: 14, Rua: 2, FracaoFechadas: 0.15,
	},
	"estresse": {
		Nome: "estresse", Largura: 2000, Altura: 2000, Ambulancias: 100, Hospitais: 20,
		ChamadosPorTurno: 1000, PrazoMin: 100, PrazoMax: 1000, Turnos: 1000,
		LimiteTurnoMs: 100, LimitePreparoMs: 2000, MemoriaMB: 512,
		FracaoGraves: 0.05, ValorGraveMin: 60, ValorComumMax: 20, PrazoGraveMin: 30, PrazoGraveMax: 150,
		Focos: 8, FracaoFocos: 0.3, RaioFoco: 40,
		QuarteiraoMin: 6, QuarteiraoMax: 14, Rua: 2, FracaoFechadas: 0.15,
	},
}
