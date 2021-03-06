# v9_deployment_manager
[![CircleCI](https://circleci.com/gh/velocity-9/v9_deployment_manager.svg?style=svg)](https://circleci.com/gh/velocity-9/v9_deployment_manager)

The deployment manager receives events from GitHub as well as the v9_website to manage the state of serverless function deployments.

### Building the Deployment Manager
If you haven't used Go before the [documentation](https://golang.org/doc/install) is quite thorough.
1. Install The Dependencies Below
2. Create an env.sh script([example](https://github.com/velocity-9/v9_deployment_manager/blob/master/docs/example_env.sh))
3. `go build`
4. `chmod +x ./env.sh`
5. `./v9_deployment_manager`

### Dependencies
- https://github.com/google/uuid
- https://github.com/src-d/go-git
- https://godoc.org/golang.org/x/crypto/ssh
- https://github.com/bramvdbogaerde/go-scp
- https://github.com/hjaensch7/webhooks/
- https://github.com/hashicorp/go-getter
- https://github.com/lib/pq
- https://golang.org/pkg/crypto/

### Indirect Dependencies
- https://github.com/kr/pretty
- https://github.com/sergi/go-diff
- https://golang.org/pkg/net/
- https://github.com/golang/sys


