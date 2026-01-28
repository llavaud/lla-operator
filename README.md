# lla-operator

## Structure du repository

```text
lla-operator/
├── api/v1alpha1/          # Définition de la Custom Resource (CRD)
├── cmd/                   # Point d'entrée de l'opérateur
├── config/                # Manifests Kubernetes (Kustomize)
├── internal/controller/   # Logique de réconciliation
├── hack/                  # Utilitaires de dev
├── test/                  # Tests e2e
├── bin/                   # Binaires téléchargés
├── Dockerfile             # Build de l'image
├── Makefile               # Commandes de build
├── PROJECT                # Métadonnées operator-sdk
└── go.mod                 # Dépendances Go
```

## Operator-sdk et Kubebuilder

Ce projet est généré avec **operator-sdk**, qui est construit au-dessus de **kubebuilder**.

### Hiérarchie des outils

```text
┌─────────────────────────────────────┐
│          operator-sdk               │  ← Red Hat / OperatorFramework
│  (fonctionnalités supplémentaires)  │
├─────────────────────────────────────┤
│           kubebuilder               │  ← Kubernetes SIG
│    (scaffolding + controller-gen)   │
├─────────────────────────────────────┤
│       controller-runtime            │  ← Kubernetes SIG
│   (bibliothèque Go pour contrôleurs)│
└─────────────────────────────────────┘
```

### Kubebuilder

Projet officiel Kubernetes (SIG API Machinery) qui fournit :

- Le **scaffolding** de base (génération de la structure du projet)
- **controller-gen** : génère CRDs, RBAC, webhooks depuis les markers Go
- Les conventions et bonnes pratiques

### Operator-sdk

Projet Red Hat (OperatorFramework) qui **enveloppe kubebuilder** et ajoute :

- Support pour Ansible et Helm (pas que Go)
- Intégration OLM (Operator Lifecycle Manager)
- Commandes `bundle`, `scorecard` pour la publication sur OperatorHub

### Lequel choisir ?

| Critère | Kubebuilder | Operator-sdk |
|---------|-------------|--------------|
| Go uniquement | Suffisant | Suffisant |
| Ansible/Helm | Non | Oui |
| Publier sur OperatorHub | Manuel | Intégré (OLM) |
| Maintenance | Kubernetes SIG | Red Hat |

En pratique, les commandes `kubebuilder init` et `operator-sdk init` génèrent quasiment le même code pour les projets Go.

## Les dossiers clés

### `api/v1alpha1/` - Définition de la Custom Resource

| Fichier | Rôle |
|---------|------|
| `lla_types.go` | Définit `LlaSpec` (état désiré) et `LlaStatus` (état observé) |
| `groupversion_info.go` | Déclare le groupe API (`mygroup.example.com/v1alpha1`) |
| `zz_generated.deepcopy.go` | Auto-généré pour la sérialisation |

### `cmd/main.go` - Point d'entrée

Initialise le **Manager** qui orchestre tout : client Kubernetes, cache, webhooks, métriques, et démarre le contrôleur.

### `internal/controller/` - Logique métier

| Fichier | Rôle |
|---------|------|
| `lla_controller.go` | Fonction `Reconcile()` appelée à chaque changement de ressource Lla |
| `lla_controller_test.go` | Tests unitaires |

### `config/` - Manifests Kubernetes

| Sous-dossier | Contenu |
|--------------|---------|
| `crd/` | Manifest CRD généré automatiquement |
| `rbac/` | ServiceAccount, Role, RoleBinding |
| `manager/` | Deployment du contrôleur |
| `default/` | Kustomization principale qui assemble tout |
| `samples/` | Exemple de ressource Lla |

> **Note :** En résumé : tu définis ta ressource dans api/, tu écris ta logique dans internal/controller/, et config/ contient tout ce qu'il faut pour déployer sur Kubernetes.

## Workflow typique

```text
# 1. Modifier les champs de la CR
#    → api/v1alpha1/lla_types.go

# 2. Regénérer les manifests
make generate && make manifests

# 3. Implémenter la logique de réconciliation
#    → internal/controller/lla_controller.go

# 4. Tester et déployer
make test
make docker-build IMG=mon-registry/lla-operator:v1
make deploy IMG=mon-registry/lla-operator:v1
```

## Multi-Controller

On peut déclarer plusieurs controllers dans un même opérateur. C'est courant quand on gère plusieurs Custom Resources.

### Créer une nouvelle API (CRD + Controller)

```bash
operator-sdk create api --group mygroup --version v1alpha1 --kind AnotherResource --resource --controller
```

Cela génère :

- `api/v1alpha1/anotherresource_types.go` - la nouvelle CR
- `internal/controller/anotherresource_controller.go` - le nouveau controller

### Enregistrer les controllers dans main.go

Dans `cmd/main.go`, chaque controller est enregistré séparément :

```go
// Controller 1
if err = (&controller.LlaReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    // ...
}

// Controller 2
if err = (&controller.AnotherResourceReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    // ...
}
```

### Architecture

```text
┌─────────────────────────────────────────────────────┐
│                     MANAGER                         │
│                                                     │
│  ┌─────────────────┐    ┌──────────────────┐        │
│  │ LlaReconciler   │    │ AnotherReconciler│        │
│  │ Watch: Lla      │    │ Watch: Another   │        │
│  │ WorkQueue A     │    │ WorkQueue B      │        │
│  └─────────────────┘    └──────────────────┘        │
│           │                      │                  │
│           └──────────┬───────────┘                  │
│                      ▼                              │
│              Client partagé                         │
│              Cache partagé                          │
│              Scheme commun                          │
└─────────────────────────────────────────────────────┘
```

Chaque controller :

- A sa propre **WorkQueue**
- Watch ses propres ressources
- A sa propre fonction `Reconcile()`

Mais ils **partagent** :

- Le même client Kubernetes
- Le même cache (SharedInformerCache)
- Le même Scheme

## Cycle de vie du Controller

### 1. Déploiement du Controller

```bash
kubectl apply -f config/manager/manager.yaml
```

```text
┌─────────────────────────────────────────────────────┐
│  Kubernetes crée un Pod avec le controller          │
│  → Le binaire Go démarre (main.go)                  │
└─────────────────────────────────────────────────────┘
```

### 2. Création du Manager

```go
// Dans main.go
mgr, _ := ctrl.NewManager(cfg, ctrl.Options{...})
```

```text
┌─────────────────────────────────────────────────────┐
│  Manager créé avec :                                │
│  - Un client REST vers l'API Server                 │
│  - Un cache vide (SharedInformerFactory)            │
│  - Le Scheme (types connus)                         │
└─────────────────────────────────────────────────────┘
```

### 3. Enregistrement du Controller

```go
// Dans main.go
(&controller.LlaReconciler{...}).SetupWithManager(mgr)
```

```text
┌─────────────────────────────────────────────────────┐
│  SetupWithManager() fait :                          │
│                                                     │
│  1. Crée un Controller avec sa WorkQueue            │
│  2. Demande au Manager un Informer pour "Lla"       │
│     → Le Manager note "il faudra un Informer Lla"   │
│  3. Enregistre un EventHandler sur cet Informer     │
│     (le lien Informer → WorkQueue)                  │
│  4. Associe Reconcile() au Controller               │
└─────────────────────────────────────────────────────┘
```

> **Note :** L'Informer n'est pas encore démarré à ce stade !

### 4. Démarrage du Manager

```go
// Dans main.go
mgr.Start(ctx)
```

C'est ici que tout démarre vraiment :

```text
┌─────────────────────────────────────────────────────────────────┐
│  mgr.Start() déclenche :                                        │
│                                                                 │
│  A) Démarrage des Informers                                     │
│     ┌─────────────────────────────────────────────────────────┐ │
│     │ Pour chaque type enregistré (Lla) :                     │ │
│     │                                                         │ │
│     │ 1. LIST initial sur l'API Server                        │ │
│     │    GET /apis/mygroup.example.com/v1alpha1/lla           │ │
│     │    → Remplit le cache avec toutes les Lla existantes    │ │
│     │                                                         │ │
│     │ 2. WATCH établi (connexion HTTP persistante)            │ │
│     │    GET /apis/mygroup.example.com/v1alpha1/lla?watch=true│ │
│     │    → Reçoit les events en streaming                     │ │
│     └─────────────────────────────────────────────────────────┘ │
│                                                                 │
│  B) Démarrage des Workers du Controller                         │
│     → Goroutines qui lisent la WorkQueue en boucle              │
│                                                                 │
│  C) Sync initial                                                │
│     → Chaque objet du LIST génère un event ADDED                │
│     → Tous passent dans la WorkQueue → Reconcile()              │
└─────────────────────────────────────────────────────────────────┘
```

### 5. Arrivée d'un event (vie normale)

Quelqu'un fait :

```bash
kubectl apply -f ma-lla.yaml
```

```text
┌──────────────┐
│  API Server  │ ──── Event ADDED {type: Lla, name: "ma-lla"} ────┐
└──────────────┘                                                   │
                                                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  INFORMER (Lla)                                                     │
│                                                                     │
│  1. Reçoit l'event via le WATCH (connexion HTTP streaming)          │
│  2. Met à jour son cache local (store)                              │
│  3. Appelle les EventHandlers enregistrés                           │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│  EVENT HANDLER (enregistré par SetupWithManager)                    │
│                                                                     │
│  func(obj) {                                                        │
│      key := "default/ma-lla"    // namespace/name                   │
│      queue.Add(key)             // pousse dans la WorkQueue         │
│  }                                                                  │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│  WORKQUEUE                                                          │
│                                                                     │
│  [ "default/ma-lla" ] ◄─── dédoublonnage + rate limiting            │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│  WORKER (goroutine)                                                 │
│                                                                     │
│  for {                                                              │
│      key := queue.Get()        // bloque jusqu'à avoir une clé      │
│      Reconcile(ctx, Request{NamespacedName: key})                   │
│      queue.Done(key)                                                │
│  }                                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Résumé controller

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                              TON OPERATOR                                   │
│                                                                             │
│  main.go:                                                                   │
│  ────────                                                                   │
│  1. cfg := ctrl.GetConfig()           // REST Config (auth + endpoint)      │
│                                                                             │
│  2. scheme := runtime.NewScheme()     // Registre vide                      │
│     clientgoscheme.AddToScheme()      // + types standards                  │
│     mygroupv1alpha1.AddToScheme()     // + ton type Lla                     │
│                                                                             │
│  3. mgr := ctrl.NewManager(cfg, Options{Scheme: scheme})                    │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │ Manager créé avec :                                             │     │
│     │ - REST client configuré                                         │     │
│     │ - Scheme avec tous les types                                    │     │
│     │ - SharedInformerCache (vide pour l'instant)                     │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│  4. (&LlaReconciler{}).SetupWithManager(mgr)                                │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │ - Crée Controller avec WorkQueue                                │     │
│     │ - Enregistre besoin d'un Informer Lla (pas encore créé)         │     │
│     │ - Configure EventHandler (Informer → WorkQueue)                 │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│  5. mgr.Start(ctx)                                                          │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │ A. Crée les Informers demandés                                  │     │
│     │    - Informer Lla créé dans le SharedInformerCache              │     │
│     │                                                                 │     │
│     │ B. Démarre les Informers                                        │     │
│     │    - LIST /apis/mygroup.example.com/v1alpha1/lla                │     │
│     │    - Remplit le cache (Indexer)                                 │     │
│     │    - WATCH établi (HTTP streaming)                              │     │
│     │                                                                 │     │
│     │ C. Attend que les caches soient synchronisés                    │     │
│     │    (HasSynced = true quand LIST terminé)                        │     │
│     │                                                                 │     │
│     │ D. Démarre les workers des Controllers                          │     │
│     │    - Goroutines qui lisent la WorkQueue                         │     │
│     │    - Appellent Reconcile() pour chaque clé                      │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Serveurs HTTP du Manager

Quand `mgr.Start()` est appelé, le Manager démarre automatiquement plusieurs serveurs HTTP :

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SERVEURS HTTP DU MANAGER                            │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  HEALTH PROBES SERVER (:8081)                                       │    │
│  │                                                                     │    │
│  │  /healthz  → Liveness probe  (le pod est-il vivant ?)               │    │
│  │  /readyz   → Readiness probe (le pod peut-il recevoir du trafic ?)  │    │
│  │                                                                     │    │
│  │  Utilisé par Kubernetes pour gérer le lifecycle du pod              │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  METRICS SERVER (:8443 HTTPS ou :8080 HTTP)                         │    │
│  │                                                                     │    │
│  │  /metrics  → Métriques Prometheus                                   │    │
│  │                                                                     │    │
│  │  Expose : reconcile_total, reconcile_errors, workqueue_depth, etc.  │    │
│  │  Désactivé par défaut (metricsAddr = "0")                           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  WEBHOOK SERVER (:9443 HTTPS)                                       │    │
│  │                                                                     │    │
│  │  Admission webhooks (si configurés) :                               │    │
│  │  - Validating : rejette les ressources invalides                    │    │
│  │  - Mutating   : modifie les ressources avant création               │    │
│  │                                                                     │    │
│  │  Nécessite des certificats TLS (cert-manager recommandé)            │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Configuration dans main.go

```go
// Health probes (toujours actif)
HealthProbeBindAddress: ":8081",
mgr.AddHealthzCheck("healthz", healthz.Ping)
mgr.AddReadyzCheck("readyz", healthz.Ping)

// Metrics server
metricsServerOptions := metricsserver.Options{
    BindAddress:   metricsAddr,      // "0" = désactivé, ":8443" = HTTPS, ":8080" = HTTP
    SecureServing: secureMetrics,    // true = HTTPS avec auth
}

// Webhook server
webhookServer := webhook.NewServer(webhook.Options{
    TLSOpts: webhookTLSOpts,         // Configuration TLS
})
```

### Résumé des endpoints

| Port | Endpoint   | Protocol | Usage                              |
|------|------------|----------|------------------------------------|
| 8081 | `/healthz` | HTTP     | Liveness probe Kubernetes          |
| 8081 | `/readyz`  | HTTP     | Readiness probe Kubernetes         |
| 8443 | `/metrics` | HTTPS    | Métriques Prometheus (si activé)   |
| 9443 | webhooks   | HTTPS    | Admission webhooks (si configurés) |

## Détail Informer

Un Informer est composé de plusieurs sous-composants :

```text
┌─────────────────────────────────────────────────────────────────────┐
│                         INFORMER (pour Lla)                         │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │                      REFLECTOR                             │     │
│  │                                                            │     │
│  │  - ListerWatcher : sait faire LIST et WATCH sur l'API      │     │
│  │  - Connexion HTTP vers l'API Server                        │     │
│  │  - Gère la reconnexion si le watch est coupé               │     │
│  │  - Stocke le resourceVersion pour reprendre où on en était │     │
│  │                                                            │     │
│  └──────────────────────────┬─────────────────────────────────┘     │
│                             │ objets                                │
│                             ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │                      DELTA FIFO                            │     │
│  │                                                            │     │
│  │  File d'attente interne qui stocke les deltas :            │     │
│  │  - Added: {obj: Lla{name: "foo"}}                          │     │
│  │  - Updated: {old: ..., new: ...}                           │     │
│  │  - Deleted: {obj: Lla{name: "foo"}}                        │     │
│  │                                                            │     │
│  └──────────────────────────┬─────────────────────────────────┘     │
│                             │ deltas                                │
│                             ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │                       INDEXER                              │     │
│  │                                                            │     │
│  │  Cache en mémoire avec index :                             │     │
│  │                                                            │     │
│  │  Store (map):                                              │     │
│  │    "default/ma-lla" → *Lla{...}                            │     │
│  │    "prod/autre-lla" → *Lla{...}                            │     │
│  │                                                            │     │
│  │  Index (optionnel):                                        │     │
│  │    Par namespace: "default" → ["ma-lla"]                   │     │
│  │    Par label: "app=foo" → ["ma-lla", "autre-lla"]          │     │
│  │                                                            │     │
│  └──────────────────────────┬─────────────────────────────────┘     │
│                             │                                       │
│                             ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │                   EVENT HANDLERS                           │     │
│  │                                                            │     │
│  │  Liste de callbacks enregistrés :                          │     │
│  │  - Controller A : OnAdd → queue.Add(key)                   │     │
│  │  - Controller B : OnAdd → queue.Add(key)                   │     │
│  │  - ...                                                     │     │
│  │                                                            │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Tests unitaires avec envtest

Les tests unitaires du controller utilisent **envtest**, un framework qui simule un environnement Kubernetes sans avoir besoin d'un vrai cluster.

### Architecture de la simulation

```text
┌─────────────────────────────────────────────────────────────────┐
│                     Environnement de test                       │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────┐ │
│  │   etcd          │◄───│  kube-apiserver │◄───│  k8sClient  │ │
│  │  (stockage)     │    │  (API REST)     │    │  (ton code) │ │
│  └─────────────────┘    └─────────────────┘    └─────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Les binaires téléchargés

Quand tu exécutes `make setup-envtest`, l'outil télécharge **3 binaires réels** de Kubernetes dans `bin/k8s/` :

| Binaire | Rôle |
|---------|------|
| `etcd` | Base de données clé-valeur qui stocke l'état du cluster |
| `kube-apiserver` | Serveur API REST de Kubernetes |
| `kubectl` | Client CLI (pour debug) |

Ces binaires sont les **vrais composants Kubernetes**, pas des mocks.

### Ce qui est simulé vs ce qui ne l'est pas

| Composant | Présent ? | Explication |
|-----------|-----------|-------------|
| etcd | **Oui (réel)** | Stocke les ressources créées |
| kube-apiserver | **Oui (réel)** | Valide les CRDs, gère CRUD |
| controller-manager | Non | Pas de contrôleurs built-in (Deployment, etc.) |
| scheduler | Non | Pas de scheduling de Pods |
| kubelet | Non | Pas de vrais Pods/conteneurs |
| Nodes | Non | Pas de nœuds dans le cluster |

### Comment ça démarre

Dans `internal/controller/suite_test.go` :

```go
testEnv = &envtest.Environment{
    // Charge tes CRDs depuis config/crd/bases
    CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
    ErrorIfCRDPathMissing: true,
}

// Démarre etcd + kube-apiserver et retourne la config de connexion
cfg, err = testEnv.Start()
```

**Ce que `testEnv.Start()` fait concrètement :**

1. Démarre un processus `etcd` sur un port aléatoire
2. Démarre un processus `kube-apiserver` connecté à cet etcd
3. Applique automatiquement tes CRDs au cluster
4. Retourne une `rest.Config` pour s'y connecter

### Le client k8sClient

```go
k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
```

Ce client est un **vrai client Kubernetes** qui communique avec le vrai `kube-apiserver` via HTTP/REST :

```go
k8sClient.Create(ctx, resource)  // → POST /apis/mygroup.example.com/v1alpha1/llas
k8sClient.Get(ctx, name, obj)    // → GET /apis/mygroup.example.com/v1alpha1/llas/name
k8sClient.Delete(ctx, resource)  // → DELETE /apis/mygroup.example.com/v1alpha1/llas/name
```

### Structure des tests avec Ginkgo

Les tests utilisent le framework **Ginkgo/Gomega** (BDD) :

```text
┌─────────────────────────────────────────────────────────────────────┐
│  suite_test.go                                                       │
│  ─────────────                                                       │
│  BeforeSuite() → Démarre etcd + kube-apiserver + crée k8sClient     │
│  AfterSuite()  → Arrête l'environnement de test                     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  lla_controller_test.go                                              │
│  ──────────────────────                                              │
│                                                                      │
│  Describe("Lla Controller", func() {                                 │
│      Context("When reconciling a resource", func() {                 │
│          BeforeEach() → Crée une ressource Lla de test               │
│          AfterEach()  → Supprime la ressource Lla                    │
│          It("should successfully reconcile", func() {                │
│              // Appelle Reconcile() et vérifie le résultat           │
│          })                                                          │
│      })                                                              │
│  })                                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

### Flux d'exécution d'un test

```text
1. BeforeSuite     → Démarre l'API server Kubernetes simulé
2. BeforeEach      → Crée une ressource Lla de test
3. It(...)         → Appelle Reconcile() et vérifie le résultat
4. AfterEach       → Supprime la ressource Lla
5. AfterSuite      → Arrête l'environnement de test
```

### Lancer les tests

```bash
make test
# ou directement
go test ./internal/controller/... -v
```

### Avantages d'envtest

| Avantage | Explication |
|----------|-------------|
| Validation réelle | L'API server valide tes CRDs comme en production |
| Pas de cluster requis | Pas besoin de Kind/Minikube/cluster distant |
| Rapide | Démarre en ~2-3 secondes |
| Isolé | Chaque suite de tests a son propre "cluster" |
| Reproductible | Même comportement sur CI et en local |

### Limitations

- **Pas de Pods réels** : tu ne peux pas tester le scheduling ou l'exécution de conteneurs
- **Pas de contrôleurs built-in** : un Deployment ne créera pas de ReplicaSet
- Pour ces cas, utilise les **tests e2e** avec un vrai cluster Kind → `test/e2e/`

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/lla-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/lla-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/lla-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/lla-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
operator-sdk edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

