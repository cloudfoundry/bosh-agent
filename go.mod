module github.com/cloudfoundry/bosh-agent/v2

go 1.24.0

require (
	code.cloudfoundry.org/clock v1.55.0
	code.cloudfoundry.org/tlsconfig v0.44.0
	github.com/Microsoft/hcsshim v0.13.0
	github.com/charlievieth/fs v0.0.3
	github.com/cloudfoundry/bosh-cli/v7 v7.9.16
	github.com/cloudfoundry/bosh-davcli v0.0.454
	github.com/cloudfoundry/bosh-utils v0.0.582
	github.com/cloudfoundry/gosigar v1.3.112
	github.com/containerd/cgroups/v3 v3.1.2
	github.com/coreos/go-iptables v0.8.0
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/golang/mock v1.6.0
	github.com/google/nftables v0.2.0
	github.com/google/uuid v1.6.0
	github.com/kevinburke/ssh_config v1.4.0
	github.com/masterzen/winrm v0.0.0-20250927112105-5f8e6c707321
	github.com/maxbrunsfeld/counterfeiter/v6 v6.12.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/nats-io/nats.go v1.48.0
	github.com/onsi/ginkgo/v2 v2.27.5
	github.com/onsi/gomega v1.39.0
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/pivotal/go-smtpd v0.0.0-20140108210614-0af6982457e5
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/crypto v0.47.0
	golang.org/x/net v0.49.0
	golang.org/x/sys v0.40.0
	golang.org/x/tools v0.41.0
	gopkg.in/yaml.v3 v3.0.1
	inet.af/wf v0.0.0-20221017222439-36129f591884
)

require (
	github.com/Azure/go-ntlmssp v0.1.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/ChrisTrenkamp/goxpath v0.0.0-20210404020558-97928f7e12b6 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/bodgit/ntlmssp v0.0.0-20240506230425-31973bb52d9b // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20250423223041-4ad5fea42851 // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.165 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	github.com/masterzen/simplexml v0.0.0-20190410153822-31eea3082786 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/nats-io/nkeys v0.4.14 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/pivotal-cf/paraphernalia v0.0.0-20180203224945-a64ae2051c20 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/tidwall/transform v0.0.0-20201103190739-32f242e2dbde // indirect
	go.opencensus.io v0.24.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba // indirect
	golang.org/x/exp/typeparams v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/telemetry v0.0.0-20260116145544-c6413dc483f5 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools/go/expect v0.1.1-deprecated // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260126211449-d11affda4bed // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	honnef.co/go/tools v0.6.1 // indirect
)
