# Design for Flux webhook receiver

## Motivation

`fluxd` has a couple of components which rely on polling external
systems for updates, namely:

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
   and that is not authenticated;
 - it accepts a payload particular to the flux API, rather than
   processing webhook payloads from elsewhere (which are many and
   varied).

Therefore, this design proposes a separate component which can be run
as a sidecar, or otherwise, and exposed to the internet via an
ingress (or in development scenarios using ngrok, say).

## Requirements

 * Support configuration of one or more hooks.

 * Support webhook payloads from at least:
   - GitHub
   - BitBucket cloud (there's an enterprise one with different webhook
     payloads)
   - GitLab
   - DockerHub

 * Support payload verification where it's available (e.g, GitHub puts
   a signature in an HTTP header; GitLab puts a shared secret in a
   HTTP header; etc.)

 * Each endpoint should have its own URL path and (where used) its own
   shared secret.
   * These need to be stable, but can change when e.g., the shared
     secret changes
   * It's also desirable for them to be easy to create and add; making
     a key, then supplying it to the config (and perhaps putting it in
     a secret) would be fine.

 * Some providers require you to install a hook per item; e.g.,
   DockerHub. It should be possible to make an endpoint that can be
   installed in a number of related places; but to repeat: this will
   be particular to the provider.

 * It should be possible to route through an ingress using a wildcard,
   so it doesn't need to be changed when e.g., a hook is added.
   * Similarly, it should be easy to construct the hook URL given the
     configuration and ingress rules

## Not requirements (yet)

 * GCP PubSub support (add it later)
 * other sources of notifications (?)
 * Support different API endpoints for different kinds of hook. The different
   components may (one day) have different endpoints -- e.g., if
   automation is put into its own container -- but not yet.
   * These can default to http://localhost:3030/api/flux/v11/notify,
     since that's where it'll be if running as a sidecar.
 * Forwarding notifications to a different host per endpoint (again:
   later)

## Design

In short:

 - you add a hook by creating a key pair and adding a record referring
   to it, plus its format (i.e., webhook source), to the config
 - then you provide the private key (and the config file, possibly) in
   a secret
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

### Configuration

In general providers will need specific parsing, be verified in
different ways, and pertain to a particular kind of notification, so
there's little point trying to be generic in the configuration.

The things that are needed to process an incoming webhook, for each
endpoint:

 - the distinguishing path element for the particular hook
 - the location of a shared secret, for verification
 - how to process the payload
   - how to parse it
   - how to verify it
   - how to construct a payload for the flux API `.../notify` endpoint

Proposed config format:

```
fluxRecvVersion: 1 # must be present and equal `1`
api: # if empty, default to http://localhost:3030/api/flux
endpoints:
- source: DockerHub
  key: dockerhub_rsa
- source: GitHub
  key: github_rsa
```

The keys are paths relative to the config file (so in the example,
they are files in the same directory).

## Optional elements

### Filtering

`fluxd` will ignore notifications that aren't relevant (e.g, git push
to a branch it doesn't care about), but we can also short circuit this
by specifying things of interest in the config and dropping payloads
early. (But then you have to keep the hook receiver consistent with
fluxd, if you change the branch or whatever. So maybe best not?)
