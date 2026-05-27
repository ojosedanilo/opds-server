# 📚 opds-server

Servidor [OPDS](https://opds.io/) minimalista escrito em Go. Aponte para uma pasta de livros e acesse sua biblioteca de qualquer leitor compatível — Readest, Kybook, Moon+ Reader, KOReader, etc.

Não precisa de banco de dados, arquivo de configuração nem dependências externas. Um único binário.

---

## Funcionalidades

- Detecta automaticamente todos os `.epub`, `.pdf`, `.mobi`, `.cbz` e `.cbr` da pasta
- Feed OPDS com navigation feed (raiz) e acquisition feed (livros)
- Busca por título via `/opds/search?q=...`
- Detecta automaticamente se está atrás de proxy HTTPS (Cloudflare Tunnel, nginx, Traefik…)
- Página de status HTML para conferir no browser
- Zero dependências externas — só biblioteca padrão do Go

---

## Instalação

### Baixar binário pré-compilado

Veja a página de [Releases](../../releases) e baixe o binário para o seu sistema.

### Compilar do código-fonte

```bash
git clone https://github.com/seu-usuario/opds-server
cd opds-server
go build -o opds-server .
```

Requer Go 1.21+. O binário final não precisa do Go instalado.

#### Cross-compile para outras plataformas

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o opds-server-linux .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o opds-server-mac .

# Windows
GOOS=windows GOARCH=amd64 go build -o opds-server.exe .
```

---

## Uso

```bash
# Modo mais simples — porta 8080, pasta ./books
./opds-server

# Porta e pasta customizadas
./opds-server 8080 /home/usuario/Livros
```

Coloque seus livros na pasta e acesse:

| URL | O que é |
|-----|---------|
| `http://localhost:8080/` | Página de status (browser) |
| `http://localhost:8080/opds` | Root catalog OPDS |
| `http://localhost:8080/opds/books` | Todos os livros |
| `http://localhost:8080/opds/search?q=palavra` | Busca por título |

---

## Configurar no Readest (ou outro leitor OPDS)

1. Abra o leitor e vá em **Adicionar catálogo OPDS**
2. Cole o endereço: `http://<IP-da-máquina>:8080/opds`
3. Salve — os livros aparecem automaticamente

> Para acessar de outros dispositivos na mesma rede, use o IP local da máquina (ex: `192.168.1.100`) em vez de `localhost`.

---

## Uso com Cloudflare Tunnel (ou qualquer proxy reverso HTTPS)

Nenhuma configuração extra necessária. O servidor detecta automaticamente o esquema correto via cabeçalhos de proxy, na seguinte ordem de prioridade:

1. `Forwarded: proto=https` (RFC 7239 — padrão moderno)
2. `X-Forwarded-Proto: https` (Cloudflare, nginx, AWS ALB, Traefik…)
3. `X-Forwarded-Ssl: on` (proxies mais antigos)
4. TLS nativo (quando o próprio servidor termina o TLS)
5. Fallback: `http`

Configure o tunnel apontando para `http://localhost:8080` e exponha via `https://sua.biblioteca.com`. Os links do feed OPDS já virão com `https://` automaticamente.

---

## Executar como serviço (Linux com systemd)

Crie `/etc/systemd/system/opds.service`:

```ini
[Unit]
Description=OPDS Library Server
After=network.target

[Service]
ExecStart=/opt/opds-server/opds-server 8080 /opt/opds-server/books
WorkingDirectory=/opt/opds-server
Restart=always
User=nobody

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now opds
```

---

## Formatos suportados

| Extensão | MIME Type |
|----------|-----------|
| `.epub` | `application/epub+zip` |
| `.pdf` | `application/pdf` |
| `.mobi` | `application/x-mobipocket-ebook` |
| `.cbz` | `application/x-cbz` |
| `.cbr` | `application/x-cbr` |

---

## Licença

MIT
