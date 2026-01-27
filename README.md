# lla-operator

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

