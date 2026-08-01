# Running BOSH Agent Integration Tests From Local Machine

This guide explains how to run the BOSH Agent integration tests from your local macOS machine against a remote bosh deployed VM. Be aware that running these tests shuts down the bosh agent on the VM. Turn off resurrection on the director and do not plan on `bosh ssh` working once tests are running.

The integration tests (`src/bosh-agent/integration`) are primarily designed to run in CI, but you can iterate on them locally by forwarding ports and copying the necessary compiled binaries and test assets to the remote VM.

## Prerequisites

1.  **BOSH CLI**: You must have the BOSH CLI installed and authenticated with your environment.
2.  **Target VM**: A running BOSH deployment with a VM you can SSH into (e.g., `test-bosh-agent-integration-resolute`).
3.  **Go**: Go installed on your local machine.

## Step-by-Step Guide

### 1. Start the SSH Tunnel

The Go SSH client used by the integration tests does not natively support `ProxyCommand` or `BOSH_ALL_PROXY` environment variables easily. The most robust way to connect is to establish a local port forward using `bosh ssh`.

Open a **new terminal window** and run the following command to forward your local port `2222` to the VM's port `22`.

```bash
# Replace 'test-bosh-agent-integration-resolute' with your deployment name
bosh -d test-bosh-agent-integration-resolute ssh --opts="-L" --opts="2222:127.0.0.1:22"
```

*Note: This will log you into the VM and give you a command prompt. **Leave this terminal open and running.** As long as this SSH session is active, your local port `2222` is forwarded to the VM.*

### 2. Set up the `agent_test_user`

The integration tests expect to run as a specific user named `agent_test_user`. Go back to your **main terminal window** and run this command to create the user and copy your SSH keys over so you can authenticate as them:

```bash
bosh -d test-bosh-agent-integration-resolute ssh -c "sudo useradd agent_test_user && sudo usermod -G bosh_sshers,bosh_sudoers agent_test_user && sudo usermod -s /bin/bash agent_test_user && sudo mkdir -p /home/agent_test_user/.ssh && sudo cp /home/vcap/.ssh/authorized_keys /home/agent_test_user/.ssh/authorized_keys && sudo chown -R agent_test_user:agent_test_user /home/agent_test_user/"
```

Next, copy your local SSH public key to the VM so the tests can authenticate:

```bash
# Upload your public key to the VM
bosh -d test-bosh-agent-integration-resolute scp ~/.ssh/id_rsa.pub :/tmp/id_rsa.pub

# Replace the agent_test_user's authorized_keys
bosh -d test-bosh-agent-integration-resolute ssh -c "sudo sh -c 'cat /tmp/id_rsa.pub > /home/agent_test_user/.ssh/authorized_keys' && sudo rm /tmp/id_rsa.pub"
```

### 3. Build the BOSH Agent and Fake Blobstore for Linux

You must compile the agent and the fake blobstore for Linux before copying them to the VM.

From your `resolute-rocket/src/bosh-agent` directory:

```bash
# 1. Build the BOSH Agent
GOOS=linux GOARCH=amd64 bin/build

# 2. Build the Fake Blobstore
cd integration/fake-blobstore
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build .
cd ../..
```

### 4. Copy Binaries and Assets to the VM

Now, copy the compiled binaries and the test release assets to the VM over the local tunnel you established in Step 1.

```bash
# 0. Clear known hosts
ssh-keygen -R "[127.0.0.1]:2222"

# 1. Stop the existing agent on the VM
ssh -p 2222 -i ~/.ssh/id_rsa agent_test_user@127.0.0.1 "sudo systemctl stop agent || sudo sv stop agent"

# 2. Copy the new Agent binary
scp -P 2222 -i ~/.ssh/id_rsa out/bosh-agent agent_test_user@127.0.0.1:/tmp/bosh-agent

# 3. Copy the Fake Blobstore
scp -P 2222 -i ~/.ssh/id_rsa integration/fake-blobstore/fake-blobstore agent_test_user@127.0.0.1:/tmp/fake-blobstore

# 4. Copy the Release Assets
scp -P 2222 -i ~/.ssh/id_rsa -r integration/assets/release agent_test_user@127.0.0.1:/tmp/release

# 5. Move everything into place and set permissions
ssh -p 2222 -i ~/.ssh/id_rsa agent_test_user@127.0.0.1 "sudo mv /tmp/bosh-agent /var/vcap/bosh/bin/bosh-agent && sudo chmod +x /var/vcap/bosh/bin/bosh-agent && sudo mv /tmp/fake-blobstore /home/agent_test_user/fake-blobstore && sudo chmod +x /home/agent_test_user/fake-blobstore && sudo rm -rf /home/agent_test_user/release && sudo mv /tmp/release /home/agent_test_user/release && sudo chown -R agent_test_user:agent_test_user /home/agent_test_user/release"
```

*(Note: If you use a different SSH key for BOSH, replace `~/.ssh/id_rsa` with the path to your key).*

### 5. Configure SSH for the Tests

All test commands (both the Go native SSH client and the system `scp`/`ssh` calls) read from `src/bosh-agent/integration/ssh-config`. Create or update that file to point to the local tunnel. The Go code hardcodes the host name `agent_vm` for this file:

```ssh-config
Host agent_vm
  Hostname 127.0.0.1
  Port 2222
  User agent_test_user
  IdentityFile /Users/YOUR_USER_NAME/.ssh/id_rsa
```

*(Ensure the `IdentityFile` points to the absolute path of the SSH key you use to connect).*

### 6. Run the Tests!

From the `src/bosh-agent/integration` directory:

```bash
go run github.com/onsi/ginkgo/v2/ginkgo -v .
```

All commands will look up `agent_vm` in `integration/ssh-config` and connect to `127.0.0.1:2222`, which your `bosh ssh` session will silently forward to the VM.
