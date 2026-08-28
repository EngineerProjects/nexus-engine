# Audit providers — incident sub-agent "browse" (août 2026)

Origine : un tour du sub-agent `browse` a tourné 36+ minutes, accumulé 107 101
tokens d'entrée, puis échoué — suivi de la mort de toute la connexion de la
conversation parente ("Connection error: terminated" puis "fetch failed").
L'investigation initiale a mené à 5 correctifs déjà livrés (`v1.2.16`→`v1.2.19`),
puis à un audit complet des 16 providers du SDK, qui a trouvé des problèmes
supplémentaires — certains sans rapport avec l'incident lui-même.

Ce fichier sert de todo-list pour s'assurer qu'on corrige vraiment tout.

---

## ✅ Déjà corrigé (v1.2.16 → v1.2.19)

- [x] **Titre de session généré après la première réponse au lieu d'avant**
      `internal/engine/session.go` — `generateTitleAsync` ne dépend que du
      premier message utilisateur, jamais de la réponse. Déclenché maintenant
      dès la persistance du premier message, plus après la fin du tour.
      *(`internal/runtime/state/store.go` : `SaveSession` préserve aussi le
      titre déjà généré contre un écrasement tardif par un save de routine.)*

- [x] **Timeout Codex de 120s qui coupait le streaming en plein milieu**
      `internal/providers/client.go` — `getCodexHTTPClient()` fixait
      `http.Client.Timeout: 120s`, qui couvre toute la durée de la requête
      *y compris* la lecture du corps streamé. Réutilise maintenant
      `newStreamingHTTPClient()` (pas de `Timeout`), + cookie jar Cloudflare.

- [x] **Aucune limite de temps sur les sub-agents async (`spawn_agent`/`wait_agent`)**
      `internal/agent/async.go` — `AsyncAgentManager.StartAgent` utilisait
      `context.WithCancel` (annulable mais sans échéance). Applique maintenant
      `DefaultSubAgentTimeout` (30 min) via `context.WithTimeout`.

- [x] **`spawn_agent` ignorait le budget de tours propre à chaque agent**
      `internal/tools/agents/spawn_agent.go` — `max_turns` par défaut était
      codé en dur à 10, ignorant `agentDef.MaxTurns` (35 pour `browse`).
      Résout maintenant via `resolveSpawnAgentDef`/`resolveSpawnAgentMaxTurns`.
      Ajout aussi du signal de continuation "conclus maintenant si complet",
      absent du chemin async (présent seulement côté agent synchrone).

- [x] **`browser_screenshot` sans limite de taille**
      `internal/web/browser/{types,extract}.go` — payload base64 illimité
      inséré dans le contexte de conversation. Plafonné à 2MB
      (`Config.MaxScreenshotBytes`) via `applyScreenshotByteCap` ; l'image
      reste toujours sauvegardée sur disque au-delà.

- [x] **Télémétrie Codex incomplète (tokens cachés / raisonnement)**
      `internal/providers/client_codex.go`, `internal/types/message.go` —
      `parseCodexSSEStream` ne lisait que `input_tokens`/`output_tokens`,
      ignorant `input_tokens_details.cached_tokens` et
      `output_tokens_details.reasoning_tokens` (forme Responses API).
      Ajout de `TokenUsage.ReasoningTokens` + parsing complet.

- [x] **`spawn_agent` ne documentait pas `browse`/`verify` comme types valides**
      `internal/tools/agents/spawn_agent.go` — enum/description du schéma ne
      listaient que `general-purpose`/`explore`/`plan`.

- [x] **Codex renvoyait tout l'historique à chaque itération interne d'un tour**
      `internal/engine/loop.go`, `internal/providers/client_codex.go`,
      `internal/types/api.go`, `internal/engine/state.go` — jusqu'à 100
      itérations internes par tour (`MaxIterations`), chacune resendait
      l'historique complet et croissant. Codex utilise maintenant
      `previous_response_id` (API Responses) pour ne renvoyer que les
      messages ajoutés depuis la dernière réponse, avec invalidation stricte
      (changement de provider, compaction) et scope limité à un seul tour
      (jamais persisté entre tours ou redémarrages).

---

## 🔴 Critique — systémique, affecte tous les providers

- [x] **Erreur de streaming mal classée partout, y compris toujours dans Codex** ✅ FIXÉ
      **Problème :** Le bug qu'on croyait corrigé pour Codex (une coupure
      réseau transitoire en cours de streaming classée comme erreur
      permanente, non-retryable) existait **identiquement dans les 5
      implémentations de streaming** — on n'avait supprimé que le
      déclencheur (le timeout 120s), jamais la classification erronée
      elle-même :
      - [x] `internal/providers/client_codex.go:238` — `parseCodexSSEStream`
      - [x] `internal/providers/client_anthropic.go:34,54` — `streamAnthropicResponse`
            (sert aussi Bedrock/Vertex/Foundry/WorkersAI via l'adaptateur par défaut)
      - [x] `internal/providers/client_openai.go:389,432,521` — sert OpenAI,
            MiniMax, OpenRouter, Mistral, DeepSeek, OpenCode, Kimi, Z.ai
      - [x] `internal/providers/client_gemini.go:252,278`
      - [x] `internal/providers/client_ollama.go:135,155`

      Root cause commune : `internal/types/errors.go` `IsRetryable()` ne
      traite que `ErrCodeAPIRateLimit`/`ErrCodeAPITimeout` comme retryable ;
      les 5 fichiers ci-dessus enveloppent tous leurs erreurs de lecture de
      flux sous `ErrCodeAPIResponse`, et `internal/engine/loop.go`
      `isRecoverableError` faisait confiance à `IsRetryable()` sans
      repasser par la classification réseau générique
      (`providerretry.ClassifyHTTPError`) pour une `*types.EngineError`.

      **Fix :** `isRecoverableError` (`internal/engine/loop.go`) tombe
      maintenant sur `ClassifyHTTPError` appliqué à la cause enveloppée
      (`EngineError.Err`) quand le code est `ErrCodeAPIResponse` et qu'une
      cause est présente — un seul point de correction couvre les 5
      providers d'un coup, plutôt que de dupliquer la logique dans chaque
      parser. `errors.Is(err, context.Canceled/DeadlineExceeded)` reste
      prioritaire (déjà vérifié en tête de fonction, et transparent au
      travers de `EngineError.Unwrap()`), donc une annulation de contexte
      n'est jamais retentée à tort. Tests : `TestIsRecoverableError_StreamResponseErrorFallsThroughToNetworkClassifier`
      (`internal/engine/engine_test.go`).

---

## 🔴 Providers complètement cassés

- [ ] **WorkersAI ne peut pas fonctionner**
      - [ ] `internal/providers/adapter.go:50-68` — absent de
            `adapterForProvider`, tombe sur `anthropicAdapter` par défaut
            (mauvaise forme de requête, mauvaise auth `x-api-key`)
      - [ ] `internal/providers/config.go:377-378` — `GetEndpoint` construit
            une URL différente (`BaseURL + "/" + model`), incohérente avec
            l'adaptateur utilisé
      - [ ] `internal/providers/config.go:167,403` — `BaseURL` par défaut
            (`https://workers.ai/v1/chat`) n'est pas le vrai domaine
            Cloudflare (`api.cloudflare.com/client/v4/accounts/{account_id}/ai/...`)
      - [ ] Aucun champ `account_id` dans `Config`, pourtant requis par
            l'API réelle de Cloudflare

- [ ] **Bedrock et Vertex ne peuvent pas fonctionner**
      - [ ] `internal/providers/config.go:395-410` — aucune `BaseURL` par
            défaut pour `APIProviderBedrock`/`APIProviderVertex` ; `GetBaseURL()`
            renvoie `""`, premier appel échoue avec une erreur opaque style
            "no Host in request URL"
      - [ ] `pkg/sdk/auth.go:305-308` — `AWS_ACCESS_KEY_ID` et
            `ANTHROPIC_VERTEX_PROJECT_ID` envoyés tels quels comme secret
            `x-api-key`, alors qu'aucun des deux n'est un secret Anthropic
            valide
      - [ ] `internal/providers/transport.go`/`transport/transport.go` —
            un vrai signeur AWS SigV4 existe mais est **mort** (jamais
            appelé) ; commentaire dans le code : *"AWS SigV4 signing is not
            yet implemented"*. Nécessite une vraie implémentation, pas un
            correctif ponctuel — à traiter comme un chantier séparé.
      - [ ] Ce code mort utilise aussi `NewHTTPClient(nil, 10*time.Minute)`
            (`round_tripper.go:12-18`) qui fixe `http.Client.Timeout` — le
            même bug que Codex, prêt à se réactiver si ce chemin est un
            jour câblé sans y penser.
      - [ ] `VertexTransport.CreateMessage` (non-streaming) n'envoie aucun
            header d'auth alors que `CreateMessageStream` en envoie un
            (incohérence interne, sans impact tant que le code est mort)

- [ ] **MiniMax — URL malformée, 404 garanti**
      - [x] `internal/providers/config.go:155,368-369` — `BaseURL` est déjà
            un chemin complet (`.../text/chatcompletion_v2`), mais
            `GetEndpoint` rajoutait `/chat/completions` par-dessus ✅ FIXÉ
            — `GetEndpoint` a maintenant un cas dédié `APIProviderMiniMax`
            qui renvoie `GetBaseURL()` tel quel, sans suffixe. Tests :
            `TestMiniMaxEndpointDoesNotDoubleAppendPath`, mise à jour de
            `TestProviderAdapterDispatch/minimax`.
      - [ ] `internal/providers/registry.go:134` décrit MiniMax comme
            "Anthropic-compatible" alors qu'il route réellement via
            `openAICompatAdapter` (`adapter.go:54`) — description à corriger

---

## 🟠 Bugs réels, fonctionnels mais faux

- [ ] **Anthropic ne cache jamais l'historique de conversation, seulement le system prompt + les outils**
      - [ ] `internal/types/message.go` — `TextContent`/`ToolResultContent`/
            etc. n'ont structurellement **aucun champ `CacheControl`** ; rien
            à marquer même si on voulait le faire aujourd'hui
      - [ ] `internal/providers/client.go:474-494` — `buildAnthropicRequestBody`
            envoie `req.Messages` sans aucune transformation/marquage
      - [ ] Impact : dans une longue boucle d'outils (exactement le
            scénario de l'incident), tout ce qui s'accumule (résultats
            d'outils compris) est renvoyé sans bénéfice de cache à chaque
            tour — le system prompt + les outils (déjà cachés) sont
            généralement petits comparés à l'historique qui grossit
      - [ ] Fix envisagé : ajouter `CacheControl` à `ToolResultContent`
            (et/ou marquer le dernier message avant sérialisation),
            même pattern que le cache déjà en place sur le dernier outil

- [ ] **Foundry perd son cache de system prompt silencieusement**
      - [ ] `internal/providers/client.go:484` — ne préserve les
            `SystemPromptBlocks` structurés (avec `cache_control`) que pour
            `provider == APIProviderAnthropic` ; Foundry tombe sur
            `FlattenSystemPromptBlocks` qui jette le `CacheControl`
      - [ ] Incohérent avec le cache des outils, qui lui fonctionne bien
            pour Foundry (`client.go:449`, testé)
      - [ ] Aucun test ne couvre le chemin system-block de Foundry
            (`providers_test.go:2999` ne teste qu'Anthropic)

- [ ] **Synchronisation de modèles cassée pour MiniMax et WorkersAI**
      - [ ] `internal/providers/fetch.go:71-97` — `DefaultBaseURL()` n'a pas
            de cas pour ces deux providers, tombe sur `default: return ""`
      - [ ] Sans URL de base explicite fournie par l'appelant, `FetchModels`
            construit `"" + "/v1/models"` → découverte de modèles cassée

- [ ] **DeepSeek — fenêtres de contexte probablement inversées**
      - [ ] `internal/providers/registry.go:174` vs `:178` — `deepseek-chat`
            à `ContextWindow: 64000` alors que `deepseek-coder-v2` (modèle
            apparemment obsolète, fusionné dans `deepseek-chat`) est à
            `128000` — un modèle phare plus récent ne devrait pas avoir une
            fenêtre plus petite qu'une variante qu'il a remplacée
      - [ ] Confiance modérée : `deepseek-chat`/`deepseek-reasoner` sont
            généralement documentés à 128K, pas 64K — à vérifier contre la
            doc officielle DeepSeek avant de changer

- [x] **Ollama — timeout de 60s potentiellement trop court pour un gros modèle local** ✅ FIXÉ
      **Problème :** `ResponseHeaderTimeout: 60s` était partagé par tous les
      providers via `newStreamingHTTPClient()`, sans override pour Ollama —
      un modèle local volumineux qui charge depuis le disque ou tourne sur
      CPU peut légitimement dépasser 60s avant le premier octet.
      **Fix :** nouveau `httpClientForProvider()` dispatche Ollama vers
      `ollamaResponseHeaderTimeout` (5 minutes) au lieu du
      `defaultResponseHeaderTimeout` (60s) partagé par les autres
      providers ; Codex garde son chemin dédié (`getCodexHTTPClient`,
      cookie jar Cloudflare). Test :
      `TestHTTPClientForProvider_OllamaGetsLongerResponseHeaderTimeout`.

---

## 🟡 Confiance plus faible / mineur

- [ ] **OpenRouter — drapeau `SupportsPC` mort**
      `internal/providers/registry.go:155` (provider-level `true`) vs
      `:157-163` (aucun des 3 modèles listés ne l'a) — le flag est affirmé
      mais jamais câblé à un modèle concret.

- [ ] **Kimi — `SupportsPC: false` possiblement faux**
      `internal/providers/registry.go:280-303` — tous les modèles Kimi sont
      à `false`, alors que l'API Moonshot réelle supporterait le context
      caching. Même classe de lacune que le bug de caching Codex déjà
      corrigé, mais confiance plus faible (pas vérifié contre la doc
      officielle).

- [ ] **Mistral — fenêtre de contexte possiblement obsolète**
      `internal/providers/registry.go:108` — `mistral-small-latest` à
      `32768`, alors que les générations récentes de Mistral Small sont
      généralement documentées à ~128K.

---

## 🧟 Code mort à surveiller (pas un bug actif, mais un piège pour plus tard)

- [ ] `internal/providers/circuit_breaker.go:186` — `ExecuteWithTimeout`
      n'est appelé nulle part aujourd'hui, mais applique un
      `context.WithTimeout` (30s par défaut) sur toute la fonction passée —
      si quelqu'un le câble un jour sur un appel de streaming sans y
      penser, ça réintroduit exactement le bug Codex (mais à 30s au lieu de
      120s, encore pire).
- [ ] `internal/providers/config.go` — `Config.BuildAuthHeaders()` n'est
      appelé nulle part ; la vraie logique d'auth vit dans
      `adapter.go`'s `applyAuthHeaders`. À supprimer ou à documenter comme
      tel pour éviter la confusion.
- [ ] `internal/providers/transport/transport.go`,
      `internal/providers/transport.go` — scaffolding Bedrock/Vertex mort,
      SigV4 non implémenté, et son propre bug de timeout latent (voir
      section Bedrock/Vertex ci-dessus).

---

## Notes de scope

- Les points **Bedrock/Vertex** (signature SigV4/OAuth GCP réelle) sont un
  chantier à part entière — implémentation complète nécessaire, pas un
  correctif ponctuel, et impossible à tester sans identifiants cloud réels.
- Les points de confiance "modérée/faible" (fenêtres de contexte, flags
  `SupportsPC`) doivent être vérifiés contre la documentation officielle de
  chaque provider avant de changer une valeur — le risque d'erreur inverse
  (remplacer une valeur correcte par une fausse) existe.
