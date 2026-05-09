# 🚀 WhatsgoAPI Standalone

Backend de alta performance para WhatsApp desenvolvido em Go utilizando a biblioteca [whatsmeow](https://github.com/tulir/whatsmeow). Esta é a versão standalone, removida do projeto `ravena-ai`, pronta para ser utilizada como uma API REST independente.

## 💾 Docker Stack
A stack é composta por:
- **whatsgoapi**: A ponte REST API em Go. Gerencia conexões, envia/recebe mensagens e webhooks.
- **postgres**: Banco de dados PostgreSQL para persistência de sessões e dados.
- **minio**: Storage de objetos (S3 compatible) para armazenamento de mídias (fotos, vídeos, áudios).

## 🐦‍⬛ Como usar

### 1. Requisitos
- [Docker](https://docs.docker.com/engine/install/) e [Docker Compose](https://docs.docker.com/compose/install/) instalados.
- [Node.js](https://nodejs.org/) (opcional, apenas para rodar o script de teste).

### 2. Configuração
Clone o repositório e configure as variáveis de ambiente:

```bash
# Gere o .env com segredos automáticos
make setup
```

Isso cria o arquivo `.env` a partir do `.env.example` e preenche automaticamente `GLOBAL_API_KEY`, `POSTGRES_PASSWORD` e `MINIO_SECRET_KEY` com valores aleatórios seguros. Se preferir fazer manualmente:

```bash
cp .env.example .env
```


### 3. Subir os containers
Utilize o `Makefile` para facilitar o gerenciamento:

```bash
# Sobe todos os serviços em background
make up
```

Após subir, você pode acessar o Swagger para ver a documentação da API:
👉 **http://localhost:9800/swagger/index.html**

> [!TIP]
> Não esqueça de clicar em **Authorize** e informar a sua `GLOBAL_API_KEY` para testar os endpoints diretamente pelo navegador.

---

## 🧪 Testando a API

Para testar a API rapidamente sem configurar um cliente complexo, utilize o script `query-whatsgo.js`.

### Preparação do script
```bash
# Instale as dependências (necessário apenas uma vez)
npm install
```

### Exemplos de uso
O script permite chamar qualquer função da API via linha de comando. Substitua `meubot` pelo identificador da instância que deseja usar.

```bash
# 1. Criar e Conectar uma nova instância rapidamente
# Formato: node query-whatsgo.js <NOME> instanceCreate <TELEFONE> <PORTA_WEBHOOK> [pair]
node query-whatsgo.js meubot instanceCreate 5511987654321 3000

# 2. Verificar o status da instância
node query-whatsgo.js meubot instanceStatus

# 3. Deletar uma instância
node query-whatsgo.js meubot instanceDelete

# 4. Enviar uma mensagem de texto
node query-whatsgo.js meubot sendText 5511999999999@s.whatsapp.net "Olá de WhatsgoAPI!"
```

### 🔗 Gerenciamento de Instâncias e Webhooks

Para testar o fluxo de eventos (mensagens recebidas, status de conexão, etc.), você pode usar o listener incluso:

1. **Inicie o listener em um terminal** (opcional):
   ```bash
   node listen-whatsgo.js 3322
   ```

2. **Crie a instância**:
   ```bash
   # Com webhook (apontando para o listener acima):
   node query-whatsgo.js meubot instanceCreate 5511987654321 3322

   # Sem webhook (padrão se omitir a porta):
   node query-whatsgo.js meubot instanceCreate 5511987654321
   ```

3. **Conexão via Pairing Code (sem QR Code)**:
   Se preferir usar o código de pareamento de 8 dígitos (o parâmetro da porta é opcional, use `0` para pular):
   ```bash
   node query-whatsgo.js meubot instanceCreate 5511987654321 0 pair
   ```

> [!TIP]
> O argumento da porta no `instanceCreate` é opcional. Se não for informado, a instância será criada **sem webhook** configurado.

> [!IMPORTANT]
> **Dica para números brasileiros**:
> O WhatsApp lida de forma inconsistente com o nono dígito. Se encontrar problemas para conectar ou receber pairing codes, tente:
> - Com o 9: `5511987654321`
> - Sem o 9: `551187654321`
> Tente ambas as variações se a primeira não funcionar imediatamente.

---

## 🛠️ Comandos do Makefile

- `make setup`: Gera o arquivo `.env` com chaves aleatórias de 30 caracteres.
- `make up`: Inicia todos os containers.
- `make up-build`: Inicia e reconstrói as imagens.
- `make down`: Para e remove os containers (mantém volumes).
- `make down-v`: Para e remove containers E volumes (reseta o banco de dados).
- `make restart`: Reinicia o container da API.
- `make logs`: Acompanha os logs da API em tempo real.
- `make build`: Reconstrói a imagem da API.
- `make docs`: Gera a documentação Swagger (requer `swag`).

## ⚠️ Importante
A `GLOBAL_API_KEY` é gerada automaticamente pelo `make setup`. Você pode encontrá-la no seu arquivo `.env` e deve utilizá-la em todos os headers das requisições para autorização (`apiKey`).
