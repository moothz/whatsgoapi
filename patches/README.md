# Patches e Customizações do `whatsmeow-lib`

Esta pasta contém os patches e a documentação técnica das modificações mantidas pela Ravena sobre a biblioteca upstream `go.mau.fi/whatsmeow` (`https://github.com/tulir/whatsmeow`), versionada no fork `https://github.com/moothz/whatsmeow`.

---

## 📦 Índice de Patches

### 1. Funcionalidades de Protocolo (Upstream-ready / PR)
1. **`0001-newsletter-add-AcceptNewsletterAdminInvite-method.patch`**
   - Adiciona a constante `mutationAcceptNewsletterAdminInvite = "6179636105471882"` e o método `AcceptNewsletterAdminInvite(ctx, jid)` em `newsletter.go` para aceitar convites de administração em canais/newsletters.
2. **`0002-appstate-add-BuildClearChat-patch-builder.patch`**
   - Adiciona a função `BuildClearChat(target, lastMessageTimestamp, lastMessageKey, deleteMedia)` em `appstate/encode.go` (e teste unitário em `appstate/encode_test.go`) para sincronização de limpeza de conversas via AppState.

### 2. Otimizações de Performance e Resiliência (Específicas para Multi-Instância / Ravena)
> **Nota:** Os patches `0003` e `0004` são voltados à operação de alta concorrência da Ravena (múltiplas instâncias compartilhando SQLite sob alto volume de mensagens e grupos) e **não** devem ser enviados como PR para o repositório oficial sem adaptação.

3. **`0003-perf-sqlstore-otimizar-queries-de-MigratePNToLID-par.patch`** (`store/sqlstore/store.go`)
   - **Problema 1 (Full-partition scan em `MigratePNToLID`):** O `whatsmeow` executava 6 queries `DELETE`/`INSERT ... SELECT` com `WHERE our_jid=$1 AND their_id LIKE $2 || ':%'` dentro de uma transação de escrita (`BEGIN IMMEDIATE`). No SQLite, o operador `LIKE` é *case-insensitive* por padrão e **não utiliza índices B-Tree** em colunas `TEXT`, causando varredura de centenas de milhares de linhas sob lock exclusivo na primeira mensagem de cada usuário após reconexão.
   - **Solução 1:** Substituição de `LIKE $2 || ':%'` por comparação de intervalo lexicográfico (`>= $2 || ':' AND < $2 || ';'`, já que `';'` é o caractere ASCII imediatamente seguinte a `':'`), garantindo *range scan* direto no índice primário B-Tree, além de uma pré-verificação rápida read-only (`hasPNToMigrateQuery` com `SELECT EXISTS`) antes de abrir `s.db.DoTxn`:
     ```go
     hasPNToMigrateQuery = `
         SELECT EXISTS(
             SELECT 1 FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id >= $2 || ':' AND their_id < $2 || ';'
             UNION ALL
             SELECT 1 FROM whatsmeow_identity_keys WHERE our_jid=$1 AND their_id >= $2 || ':' AND their_id < $2 || ';'
             UNION ALL
             SELECT 1 FROM whatsmeow_sender_keys WHERE our_jid=$1 AND sender_id >= $2 || ':' AND sender_id < $2 || ';'
         )
     `
     ```
   - **Problema 2 (Crescimento ilimitado e I/O em `whatsmeow_message_secrets`):** Cada mensagem enviada/recebida e cada pacote de `HistorySync` inseria registros em `whatsmeow_message_secrets` sem política de expurgo (acumulando >34 milhões de linhas / ~5,8 GB em produção e travando o SQLite por 40s–90s).
   - **Solução 2:** Substituição da persistência em disco em `PutMessageSecrets`, `PutMessageSecret` e `GetMessageSecret` por um *ring-buffer* em memória (`msgSecretCache`, limitado a `maxCachedMsgSecrets = 20000` por instância):
     ```go
     type cachedMsgSecret struct {
         secret []byte
         sender types.JID
     }

     const maxCachedMsgSecrets = 20000
     ```
   - **Correção adicional:** `DeleteIdentity` corrigido para executar `deleteIdentityQuery` (`their_id=$2`) em vez de `deleteAllIdentitiesQuery`.

4. **`0004-fix-client-abortar-envios-expirados-na-fila-de-lock-.patch`** (`send.go`, `keepalive.go`)
   - **Problema 1 (Envios fantasmas/atrasados após timeout HTTP):** Em `Client.SendMessage` (`send.go`), todas as chamadas aguardam `cli.messageSendLock.Lock()`. Quando a fila atrasava e o cliente HTTP (ou contexto de 50s) cancelava a requisição, o `whatsmeow` prosseguia com a criptografia e envio ao adquirir o lock, entregando mensagens minutos depois (ou duplicadas com os retries).
   - **Solução 1 (`send.go`):** Checagem imediata de `ctx.Err()` logo após adquirir `cli.messageSendLock.Lock()`:
     ```go
     cli.messageSendLock.Lock()
     resp.DebugTimings.Queue = time.Since(start)
     defer cli.messageSendLock.Unlock()
     if err = ctx.Err(); err != nil {
         return
     }
     ```
   - **Problema 2 (Demora na detecção de conexões WebSocket zumbis / *half-open*):** Reduzido `KeepAliveMaxFailTime` em `keepalive.go` de `3 * time.Minute` para `90 * time.Second`:
     ```go
     KeepAliveMaxFailTime = 90 * time.Second
     ```

---

## 🔄 Como Atualizar o Fork com o Repositório Oficial (`tulir/whatsmeow`)

1. **Buscar novidades do upstream e fazer rebase dos commits customizados:**
   ```bash
   cd whatsmeow-lib
   git fetch origin
   git rebase origin/main
   git push fork main --force-with-lease
   ```

2. **Re-gerar os arquivos `.patch` após um rebase (se necessário):**
   ```bash
   git -C whatsmeow-lib format-patch origin/main..HEAD -o ../patches
   ```

3. **Aplicar os patches manualmente em um clone limpo do `whatsmeow`:**
   ```bash
   git -C whatsmeow-lib am ../patches/*.patch
   ```
