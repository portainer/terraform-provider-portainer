# D2K environment

Requires Portainer Business Edition 3.0 or later.

Installs d2k in the namespace `d2k-team-a` of a Kubernetes environment and registers the D2K environment that serves the Docker API from it, then creates a Docker network through that environment.

The Kubernetes environment must be connected through an Edge Agent 3.0.0 or later in tunnel (standard) mode; Async Edge Agents, Portainer Agents and local connections cannot run d2k.

`terraform destroy` removes the D2K environment from Portainer only: the d2k install and its workloads keep running in the `d2k-team-a` namespace until you delete it.
