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

- [x] **WorkersAI ne peut pas fonctionner** ✅ FIXÉ
      **Vérifié directement sur `developers.cloudflare.com/workers-ai` (28/08) :**
      Cloudflare expose un vrai endpoint OpenAI-compatible —
      `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/v1/chat/completions`,
      auth `Authorization: Bearer {token}` — donc pas besoin d'un
      adaptateur sur-mesure, juste router vers l'adaptateur OpenAI déjà
      partagé par 7 autres providers.
      - [x] `internal/providers/adapter.go` — `adapterForProvider` route
            maintenant WorkersAI vers `openAICompatAdapter` (au lieu de
            tomber sur `anthropicAdapter` par défaut, forme de requête et
            header d'auth faux)
      - [x] `internal/providers/config.go` — `GetEndpoint` construit
            maintenant `{BaseURL}/accounts/{account_id}/ai/v1/chat/completions`
            (le modèle va dans le corps JSON, pas dans l'URL, comme tout
            provider OpenAI-compatible)
      - [x] `BaseURL` par défaut corrigé vers le vrai domaine Cloudflare
            (`https://api.cloudflare.com/client/v4`)
      - [x] `account_id` ajouté — réutilise le champ `ProjectID` déjà
            existant (même rôle que le project id GCP de Vertex : un
            identifiant de compte/tenant intégré directement dans l'URL,
            pas juste un header), câblé de bout en bout :
            - Interne : `internal/providers/config.go`'s `DefaultConfigs()`
              lit `CLOUDFLARE_ACCOUNT_ID` directement (même pattern que
              `ANTHROPIC_VERTEX_PROJECT_ID` pour Vertex) ;
              `ValidateProviderConfig` l'exige.
            - Publique : `pkg/config/provider_catalog.go`'s
              `effectiveProviderProjectID`/`setupFieldsForProvider`/
              `setupHintForProvider`/`ApplyRuntimeEnv`/
              `ValidateProviderSetup` étendus à WorkersAI — l'assistant de
              configuration CLI expose maintenant le champ "Cloudflare
              account id" sans aucun changement dans `cmd/cli/config.go`
              ni `cmd/cli/runtime.go` (leur câblage `provider_project_id`
              était déjà générique, pas spécifique à Vertex).
      Tests : `TestAdapterForProviderWorkersAIUsesOpenAICompat`,
      `TestWorkersAIProviderConfig`,
      `TestValidateProviderConfig_WorkersAIRequiresAccountID` (interne),
      `TestValidateProviderSetup_WorkersAIRequiresAccountID`,
      `TestWorkersAISetupFieldsIncludeAccountID` (publique).
      **Non vérifié :** la synchronisation de modèles (`fetch.go`) reste
      cassée pour WorkersAI — Cloudflare n'a pas d'endpoint `/v1/models`
      générique sans compte, et `FetchModels` n'a pas de paramètre
      account_id dans sa signature actuelle. Laissé de côté plutôt que de
      deviner un endpoint de découverte non confirmé.

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

- [x] **MiniMax — URL malformée, 404 garanti** ✅ FIXÉ
      - [x] `internal/providers/config.go:155,368-369` — `BaseURL` est déjà
            un chemin complet (`.../text/chatcompletion_v2`), mais
            `GetEndpoint` rajoutait `/chat/completions` par-dessus ✅ FIXÉ
            — `GetEndpoint` a maintenant un cas dédié `APIProviderMiniMax`
            qui renvoie `GetBaseURL()` tel quel, sans suffixe. Tests :
            `TestMiniMaxEndpointDoesNotDoubleAppendPath`, mise à jour de
            `TestProviderAdapterDispatch/minimax`.
      - [x] `internal/providers/registry.go:134` décrivait MiniMax comme
            "Anthropic-compatible" alors qu'il route réellement via
            `openAICompatAdapter` (`adapter.go:54`) ✅ FIXÉ — description
            corrigée en "OpenAI-compatible chat completions"

---

## 🟠 Bugs réels, fonctionnels mais faux

- [x] **Anthropic ne cache jamais l'historique de conversation, seulement le system prompt + les outils** ✅ FIXÉ
      **Problème :** `internal/types/message.go` — `TextContent`/
      `ToolResultContent` n'avaient structurellement **aucun champ
      `CacheControl`** ; `buildAnthropicRequestBody` envoyait `req.Messages`
      sans aucune transformation/marquage. Dans une longue boucle d'outils
      (exactement le scénario de l'incident), tout ce qui s'accumule
      (résultats d'outils compris) était renvoyé sans bénéfice de cache à
      chaque tour.
      **Fix :** `CacheControl *PromptCacheControl` ajouté à `TextContent` et
      `ToolResultContent` (marshal/unmarshal + `cloneContentBlock` mis à
      jour pour le préserver). Nouvelle `anthropicMessagesWithCacheControl`
      (`internal/providers/client.go`) marque le dernier bloc cacheable
      (texte ou résultat d'outil) du dernier message avant sérialisation,
      même pattern que le cache déjà en place sur le dernier outil —
      no-op sans mutation si le bloc final n'est pas d'un type cacheable
      (ex. `ToolUseContent`), et no-op pour les providers non compatibles
      Anthropic. Reste sous la limite de 4 breakpoints Anthropic (system +
      outils + ce nouveau point = 3). Tests :
      `TestTextAndToolResultContentCacheControlRoundTrip`,
      `TestAnthropicMessagesWithCacheControl` (6 sous-cas),
      `TestBuildRequestBody_AnthropicMessagesHaveCacheControlOnTrailingBlock`.

- [x] **Foundry perd son cache de system prompt silencieusement** ✅ FIXÉ
      **Problème :** `internal/providers/client.go:484` ne préservait les
      `SystemPromptBlocks` structurés (avec `cache_control`) que pour
      `provider == APIProviderAnthropic` ; Foundry tombait sur
      `FlattenSystemPromptBlocks` qui jette le `CacheControl`. Incohérent
      avec le cache des outils, qui lui fonctionnait déjà pour Foundry
      (`client.go:449`, testé). Aucun test ne couvrait le chemin
      system-block de Foundry.
      **Fix :** la condition inclut maintenant aussi
      `APIProviderFoundry`, qui parle le même format Anthropic Messages et
      partage ce même constructeur de requête. Test :
      `TestBuildRequestBody_FoundrySystemPromptBlocksWithCache`.

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

Les 3 points ci-dessous ont été vérifiés via recherche web (docs
officielles Moonshot/Mistral/OpenRouter + issues de projets tiers) avant
tout changement, conformément à la note de scope plus bas.

- [x] **OpenRouter — drapeau `SupportsPC` mort** ✅ FIXÉ
      **Vérifié :** OpenRouter supporte bien le prompt caching pour
      certains modèles sous-jacents (ex. Anthropic via `cache_control`),
      *mais* seulement si la requête est envoyée au format natif Anthropic
      — hors ce codebase route OpenRouter via l'adaptateur
      OpenAI-compatible (`chat/completions`), qui n'a aucun champ
      `cache_control`. Donc aucune requête envoyée par ce code n'est
      réellement cachée aujourd'hui, quel que soit le modèle choisi.
      **Fix :** `SupportsPC` provider-level passé à `false`, reflétant ce
      qui est réellement implémenté plutôt que ce qu'OpenRouter permettrait
      en théorie.

- [x] **Kimi — `SupportsPC: false` possiblement faux** ✅ FIXÉ
      **Vérifié :** l'API Moonshot cache automatiquement côté serveur —
      aucune action côté client requise (pas de `cache_control`, pas de
      TTL, rien à changer dans le code) dès que le préfixe d'une requête
      correspond à une requête précédente. Donc `true` est exact
      indépendamment de ce que fait ce codebase.
      **Fix :** `SupportsPC: true` (provider + chaque modèle) et
      `Capabilities.PromptCaching: true` pour tous les modèles Kimi.

- [x] **Mistral — fenêtre de contexte possiblement obsolète** ✅ FIXÉ
      **Vérifié :** Mistral Small 3.1 (version courante) a une fenêtre de
      128K confirmée par l'annonce officielle Mistral — `32768` correspond
      à l'ancienne génération Mistral Small 3.
      **Fix :** `mistral-small-latest` passé à `131072` (128K, même
      convention d'unité que `mistral-large-latest` dans ce fichier).

Tests pour ces 3 points :
`TestRegistryPromptCachingMetadataMatchesWhatIsActuallyImplemented`.

- [x] **DeepSeek — catalogue de modèles entièrement obsolète (IDs retirés, pas juste la fenêtre de contexte)** ✅ FIXÉ
      **Vérifié directement sur `api-docs.deepseek.com` (28/08, fetch
      direct, pas une source tierce) :** la page pricing/quick_start ne
      liste plus que `deepseek-v4-flash`, `deepseek-v4-pro`,
      `deepseek-v4-flash-vision-exp` (tous ~1M contexte / 384K output) —
      aucune mention de `deepseek-chat`/`deepseek-reasoner`/`deepseek-coder-v2`,
      qui correspondent bien aux noms **legacy
      rapportés retirés le 2026-07-24**. Ce n'était donc pas juste "la
      fenêtre de contexte est inversée" mais un catalogue entier construit
      sur des IDs de modèles qui ne résolvent probablement plus.
      **Fix :** `registry.go` et le `ModelAliasMapping` de `config.go`
      (aliases `default`/`chat`/`coder`/`reasoner`/`r1`) repointés vers les
      3 modèles V4 actuels, avec les bonnes fenêtres de contexte (1048576)
      et sorties max (393216). Test :
      `TestDeepSeekCatalogUsesCurrentModelIDs` (échoue si un ID retiré
      réapparaît, vérifie que les 3 IDs actuels sont présents avec un
      contexte ≥1M, et qu'aucun alias ne pointe vers un ID retiré). Un
      test existant (`pkg/config/config_test.go`'s
      `TestProviderForModelUsesCatalog`) utilisait `deepseek-chat` comme
      exemple réel de lookup catalogue — mis à jour vers
      `deepseek-v4-flash`.
      **Note :** l'entrée `deepseek/deepseek-r1` sous OpenRouter
      (`registry.go:169`) n'a pas été touchée — c'est un routage OpenRouter
      indépendant de l'API directe DeepSeek, hors du scope de cette
      vérification.

- [ ] **Codex — `ModelAliasMapping` pointe vers des modèles déjà confirmés cassés dans ce même fichier**
      **Découvert (28/08) en corrigeant DeepSeek, pas encore corrigé.**
      `internal/providers/config.go` (aliases `default`/`codex` → 
      `gpt-5.3-codex`, `5.2` → `gpt-5.2-codex`) pointe vers des IDs que
      `internal/providers/registry.go`'s propre commentaire qualifie de
      *"confirmed broken by a real ChatGPT-account session"* — le
      catalogue de `registry.go` liste en réalité `gpt-5.6-sol`, `gpt-5.5`,
      `gpt-5.4-mini`. Donc taper `codex:default` ou `codex:codex` envoie
      un modèle que ce même repo sait déjà ne pas fonctionner. Incohérence
      interne pure, vérifiable sans aucune recherche externe — juste
      aligner les alias sur le catalogue déjà correct.

---

## 🧟 Code mort à surveiller (pas un bug actif, mais un piège pour plus tard)

- [x] `internal/providers/circuit_breaker.go:186` — `ExecuteWithTimeout` ✅ DOCUMENTÉ
      N'est appelé nulle part aujourd'hui, mais applique un
      `context.WithTimeout` sur toute la fonction passée — si quelqu'un le
      câble un jour sur un appel de streaming sans y penser, ça
      réintroduit la même classe de bug que Codex. A une couverture de
      test substantielle (7 tests, y compris race/concurrency) suggérant
      une conception délibérée pour un usage futur — supprimer aurait été
      excessif. Un avertissement a été ajouté à son commentaire de doc au
      lieu de le supprimer.
- [x] `internal/providers/config.go` — `Config.BuildAuthHeaders()` ✅ SUPPRIMÉ
      N'était appelé nulle part (zéro référence, y compris dans les tests)
      ; la vraie logique d'auth vit dans `adapter.go`'s
      `applyAuthHeaders`. C'était une copie dupliquée et non maintenue
      (déjà désynchronisée — absence de garantie qu'elle reste correcte),
      donc supprimée plutôt que documentée.
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
