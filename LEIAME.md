# Kit do trabalho: Central de Despacho

Conteúdo:

- `bin/`: servidor do jogo, corretor e bots `ingenuo`, `indexado` e `etapa2`, já compilados;
- `ingenuo/`: código do bot ingênuo, com `go.mod`, ponto de partida do seu bot;
- `servidor-fonte/`: código-fonte do servidor;
- `visualizador/index.html`: abra no navegador e escolha um arquivo de replay.

O enunciado do trabalho não vem no kit. A versão que vale é a publicada em
https://tads-ufpr-alexkutzke.github.io/ds143-material/trabalho_2026_02/.

O servidor e o corretor rodam em qualquer sistema. A medida de tempo usada na correção, por tempo de processador, só existe em Linux. Em Windows e macOS, o servidor mede tempo de relógio, e a razão do teste de tempo fica menos precisa. Para medir como a correção mede, use o Linux, o WSL2 no Windows, ou o Docker.

## Com Docker (Windows, macOS ou Linux)

Use o kit `docker`. Na pasta do kit, uma vez:

```
docker build -t despacho .
```

Depois, para trabalhar, abra um terminal dentro do contêiner com a pasta do kit montada em `/trabalho`:

```
docker run --rm -it -v "$PWD":/trabalho despacho
```

No PowerShell do Windows, troque `"$PWD"` por `"${PWD}"`. Dentro do contêiner, o servidor, o corretor e os bots de referência estão no PATH:

```
cd ingenuo
go build -o meubot .
servidor -cenario medio -etapa 1 -semente 1 -replay partida.jsonl ./meubot indexado
corretor -etapa 1 -bin /usr/local/bin -sementes 1-10 .
```

Os arquivos gravados em `/trabalho` aparecem na pasta do kit, fora do contêiner. O replay se abre no visualizador pelo navegador do seu computador. A opção `-nucleo` do corretor não funciona no contêiner.

## Sem Docker

Use o kit do seu sistema. É preciso ter o Go instalado, porque o corretor compila o seu bot.

Linux e WSL2:

```
cd ingenuo
go build -o meubot .
../bin/servidor -cenario medio -etapa 1 -semente 1 -replay partida.jsonl ./meubot ../bin/indexado
../bin/corretor -etapa 1 -bin ../bin -sementes 1-10 .
```

macOS: os mesmos comandos. Antes da primeira execução, libere os executáveis baixados pelo navegador, ou o sistema bloqueia a abertura:

```
xattr -dr com.apple.quarantine <pasta do kit>
```

Use o kit `macos-arm64` em Mac com processador Apple (M1 em diante) e o `macos-amd64` em Mac com processador Intel.

Windows, no PowerShell:

```
cd ingenuo
go build -o meubot.exe .
..\bin\servidor.exe -cenario medio -etapa 1 -semente 1 -replay partida.jsonl .\meubot.exe ..\bin\indexado.exe
..\bin\corretor.exe -etapa 1 -bin ..\bin -sementes 1-10 .
```

O antivírus do Windows às vezes bloqueia executáveis Go sem assinatura digital. Se isso acontecer, libere a pasta do kit nas exclusões do Microsoft Defender.

## Sementes

As sementes de treino são de 1 a 10. A correção usa outras sementes, com a mesma suíte de testes.
