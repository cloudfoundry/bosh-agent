module github.com/cloudfoundry/bosh-agent

go 1.15

require (
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	github.com/ChrisTrenkamp/goxpath v0.0.0-20190607011252-c5096ec8773d // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/Microsoft/hcsshim v0.8.6
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1
	github.com/cloudfoundry/bosh-davcli v0.0.44
	github.com/cloudfoundry/bosh-init v0.0.103
	github.com/cloudfoundry/bosh-utils v0.0.0-20200429181937-96357f638e8c
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/cloudfoundry/socks5-proxy v0.2.0 // indirect
	github.com/cloudfoundry/yagnats v0.0.0-20191003180638-5cab3e099974
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/golang/mock v1.3.1-0.20190508161146-9fa652df1129
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/uuid v1.1.2-0.20190416172445-c2e93f3ae59f
	github.com/jpillora/backoff v0.0.0-20170918002102-8eab2debe79d // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190724205821-6cfae18c12b8
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/masterzen/simplexml v0.0.0-20190410153822-31eea3082786 // indirect
	github.com/masterzen/winrm v0.0.0-20190308153735-1d17eaf15943
	github.com/mitchellh/mapstructure v1.1.2
	github.com/nats-io/nats.go v1.8.1
	github.com/nats-io/nkeys v0.1.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.10.2
	github.com/onsi/gomega v1.7.0
	github.com/pivotal-cf/paraphernalia v0.0.0-20180203224945-a64ae2051c20 // indirect
	github.com/pivotal/go-smtpd v0.0.0-20140108210614-0af6982457e5
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/sys v0.0.0-20210119212857-b64e53b001e4
	golang.org/x/tools v0.1.0 // indirect
	gopkg.in/yaml.v2 v2.2.4 // indirect
)
