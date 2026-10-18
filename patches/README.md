# Patches Customizados do whatsmeow-lib

Esta pasta contém os patches das funcionalidades customizadas mantidas pela Ravena sobre a biblioteca upstream `go.mau.fi/whatsmeow` (`https://github.com/tulir/whatsmeow`).

## Patches Incluídos:
1. `0001-feat-newsletter-adicionar-metodo-AcceptNewsletterAdm.patch`:
   - Adiciona o método `AcceptNewsletterAdminInvite(ctx, jid)` para aceitar convites de administração em canais/newsletters.
2. `0002-feat-appstate-adicionar-fun-o-BuildClearChat-para-si.patch`:
   - Adiciona a função `BuildClearChat(target, lastMessageTimestamp, lastMessageKey, deleteMedia)` no pacote `appstate` para sincronização de limpeza de conversas.

## Como re-gerar ou aplicar os patches:
- **Exportar commits do whatsmeow-lib para patches:**
  ```bash
  git -C whatsmeow-lib format-patch -2 HEAD -o ../patches
  ```
- **Aplicar patches em um clone limpo do whatsmeow:**
  ```bash
  git -C whatsmeow-lib apply ../patches/*.patch
  ```

## Estratégia de Longo Prazo Recomendada (Fork no GitHub):
1. Crie um fork oficial de `tulir/whatsmeow` no seu GitHub (ex.: `github.com/moothz/whatsmeow`).
2. Adicione os 2 commits na branch `main` do seu fork.
3. Altere o remote do submodule `whatsmeow-lib` para o seu fork:
   ```bash
   git -C whatsmeow-lib remote set-url origin git@github.com:moothz/whatsmeow.git
   ```
4. Para atualizar com novidades do whatsmeow upstream:
   ```bash
   git -C whatsmeow-lib remote add upstream https://github.com/tulir/whatsmeow.git
   git -C whatsmeow-lib fetch upstream
   git -C whatsmeow-lib rebase upstream/main
   git -C whatsmeow-lib push origin main -f
   ```
5. *(Opcional)* Abra um Pull Request no repositório oficial `tulir/whatsmeow` para que essas 2 funcionalidades sejam incorporadas diretamente no upstream.
