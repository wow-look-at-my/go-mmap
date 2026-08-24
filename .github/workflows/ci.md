# ci.yml

## Why the workflow turns autorelease off

This module is a library. It has no main package, so a build produces an ar archive that nobody downloads. buildhost rejects that archive once the
cosmo target succeeds, because an archive carries no APE magic.

Consumers take this module through the Go module proxy, so there is nothing for a release to serve.
