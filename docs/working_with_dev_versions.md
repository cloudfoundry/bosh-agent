# Working with the bosh-agent

## Preface
The [bosh-agent](https://github.com/cloudfoundry/bosh-agent) resides in all bosh VMs and is responsible for communication with the director. It comes packaged within the stemcell, therefore, developing new features for the bosh-agent can be a bit tricky due to the unexplored feedback path for the changes we bring about in the code. In this article, we will explore an easy way to test our new features on VMs deployed on our development landscapes easily without having the need of packaging the agent in a new dev-stemcell.

## Ensuring the basic setup
Before starting the development flow, make sure to setup the environment as explained [here](https://github.com/cloudfoundry/bosh-agent/blob/main/docs/dev_setup.md#set-up-a-workstation-for-development).

## The dev-flow in a nutshell

The process consists of five generic steps:

1. Work on your feature in the bosh-agent code
2. Run the unit/integration tests: Run the tests as defined [here](https://github.com/cloudfoundry/bosh-agent/blob/main/docs/dev_setup.md#running-tests).
3. Create the agent binary: Assuming you are in the bosh-agent directory, run `./bin/build` to create the agent binary. The newly-built binary would be available as bosh-agent/out/bosh-agent. Apple silicon users should make sure to edit the build script and switch `CGO_ENABLED` from 0 to 1 before running the build script.
4. Copy the dev-agent binary to a bosh VM: Run `bosh -e <bosh-director> -d <deployment> scp <dev-agent> <instance_name>:/tmp`
5. Stop the previous agent in this VM and make the dev-agent as the active agent: Run `bosh -e <bosh-director> -d <deployment> ssh -c 'sudo sv stop agent && sudo cp /tmp/<dev-agent> /var/vcap/bosh/bin && sudo sv start agent'`

That's all! Now it's upto your feature how you want to go about examining it.