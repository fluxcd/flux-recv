# Design for Flux webhook receiver

## Motivation

`fluxd` has a couple of components which rely on polling external systems for updates, namely:

 - image update automation polls DockerHub etc.; and,
 - syncing polls a git repo for new commits.

In both these cases, the service being polled may also provide webhook
notifications. For example, GitHub will POST to a webhook receiver
when it gets a git push.

To make `fluxd` more responsive to particular changes, the `/notify`
endpoint was added to the flux API. But it cannot be used as a webhook
receiver itself, since:

 - to expose it to the internet (or to the cluster) would mean
   exposing the flux API to the internet (or the rest of the cluster),
   and it's not authenticated;
 - it accepts a payload particular to the flux API, rather than
   processing webhook payloads from elsewhere (which are many and
   varied).

## Requirements

 * Support configuration of one or more hooks.

 * Support webhook payloads from at least:
  - GitHub
  - BitBucket cloud (there's an enterprise one with different webhook payloads)
  - GitLab
  - DockerHub

 * Support payload verification where it's available (e.g, GitHub puts
   a signature in an HTTP header; GitLab puts a shared secret in a
   HTTP header; etc.)

 * Support different API endpoints for different hooks. The different
   components may (one day) have different endpoints, e.g., if
   automation is put into its own container.
   * These can default to http://localhost:3030/api/flux/v11/notify,
     since that's where it'll be if running as a sidecar.

 * Each endpoint should have its own URL path and (where used) its own shared secret.
  * These need to be stable
  * It's also desirable for them to be easy to create and add; making
    a key, then supplying it to the config (and perhaps putting it in
    a secret) would be fine.

 * It should be possible to route through an ingress using a wildcard,
   so it doesn't need to be changed when e.g., a hook is added.
   * Similarly, it should be easy to construct the hook URL given the
     configuration and ingress rules

## Design

In short:

 - you add a hook by creating a key pair and adding a record referring
   to it, plus its format (i.e., webhook source), to the config
 - then you provide the private key (and tghe config) in a secret
 - each key's fingerprint is used in the URL path
   - you can figure it out by calculating that yourself, or looking in
     the logs

Considerations:

 - It might not be that easy to update the secret once you've created
   it, since you have to mention all files when doing `kubectl create
   secret generic`
   - But you can use `kubectl apply -k` with a secret generator,
     potentially. See
     https://github.com/kubernetes-sigs/kustomize/issues/692 for some
     history, and
     https://kubernetes.io/docs/concepts/configuration/secret/ for a
     how-to.
   - If the config refers to keys with a path, they can be mounted
     from different secrets

## Optional elements

### Filtering

`fluxd` will ignore notifications that aren't relevant (e.g, git push
to a branch it doesn't care about), but we can also short circuit this
by specifying things of interest in the config and dropping payloads
early. (But then you have to keep the hook receiver consistent with
fluxd, if you change the branch or whatever. So maybe best not?)
