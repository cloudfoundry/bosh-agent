package integration

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"

	"github.com/cloudfoundry/bosh-agent/v2/integration/integrationagentclient"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

const agentCert = `-----BEGIN CERTIFICATE-----
MIIC3zCCAcegAwIBAgIBADANBgkqhkiG9w0BAQUFADAzMQswCQYDVQQGEwJVUzEQ
MA4GA1UECgwHUGl2b3RhbDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTIwMTIy
MTEzMloXDTE2MTIwMTIyMTEzMlowMzELMAkGA1UEBhMCVVMxEDAOBgNVBAoMB1Bp
dm90YWwxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBALra3YIx0O6PLcQsHAXbFzJye1M/Am3gCMcX5BTui+I7vfyMXG4w
FyXHLf9lWwe23ypvP4c1+LNTq7UTTZuidrnadlADgdDned0C09bOKv5Nzk466XTR
fNyBEyxaZzHQLa9SmDLgq1GUun8oEsxZ+uXhMq6kikRu5LBCChtVCW6LzG/FE1qm
jSSH6iaOwk2yQxKVqUKNPfz1PqtRgaUBjVWrh2+Wf22KzOTORouBOrSfdxep1Cjz
lQWt2W5l05dvf2vZTlaqDCk8PBF36FWPlwmZxRsHGACVuckl3yJ69jIaa+i+mK3k
cfi05ZafWeFwm21ahqzwK/kGsK1ofPHKxE8CAwEAATANBgkqhkiG9w0BAQUFAAOC
AQEAD1VzwtWCx32pQi5l0oFfjWqktnqfhs/Rr0ZpwacBBXHBvKuz9ENXmblt5pZu
JP7je+uXQD+da/oVhl0US2L0upIdMmD1utVXYHfRji5r/tIPl2SEKTrFiNZR1Wp6
J0nE/BW7nm41dXRBIAZR71yproaQrt1tFDFZvdfhwHGLC51L6toOhk/7S604sxbk
qV0tzT+VaR4hh09FEt9xGmB/3yFh329Yib8ScT94nKzSzoNoDp4Ms/smFhF4lUio
7SD2+b2/nt8Mcz7q58nYvZteipRrmkOFszlNF5dU31FjvRLITn0bhiOOFRD3qAou
cSOr1qwsAKuu6MzYNh2ubsLvfg==
-----END CERTIFICATE-----`

const agentKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAutrdgjHQ7o8txCwcBdsXMnJ7Uz8CbeAIxxfkFO6L4ju9/Ixc
bjAXJcct/2VbB7bfKm8/hzX4s1OrtRNNm6J2udp2UAOB0Od53QLT1s4q/k3OTjrp
dNF83IETLFpnMdAtr1KYMuCrUZS6fygSzFn65eEyrqSKRG7ksEIKG1UJbovMb8UT
WqaNJIfqJo7CTbJDEpWpQo09/PU+q1GBpQGNVauHb5Z/bYrM5M5Gi4E6tJ93F6nU
KPOVBa3ZbmXTl29/a9lOVqoMKTw8EXfoVY+XCZnFGwcYAJW5ySXfInr2Mhpr6L6Y
reRx+LTllp9Z4XCbbVqGrPAr+QawrWh88crETwIDAQABAoIBAFh3DrB5TWXku9JI
3+uV0uG7ec/r4QaMLxuzn/SZC/lMN6K+AXTZp9vm9UwZfIOmfPnmObmWP+0HDCBq
xy+MN5G+cI1pW6jALt4IXKsyaQCFbctz8Nux4t+y7JTvKDRZT4fWHuDXpcS2GaXi
HyRI5ZS4jfpJRH0p03PvkEFofVKsZfLAlwnx7HhPqztyc6huu7njjP6XUYbvrhOS
otE/wyyLyT36rQvUVLHfrco2Ib1r58sWb1hL84/5HnbxwsEelBAAGL5VGu2f0zLP
qxmy4XWJg9U/UKzG9gXmbcpDUn1pUcZV1rSSJ/wfbdvBSLF+dQaE6QdjGeAyrt7M
I7gCViECgYEA5ui1aXomHIwqbzll/qotebYNL8AOcpgRJTtzODcnzoe/5aA7yVox
vOFTD9rODaVRhtdC/ZHsU6rID+tx9gQdjLUEXUqS3KsPXSiwbYBPXnEtpgWz/RAu
qpxpMn3kfVbQgJjkfksWvlHNzLj5q4ZhZyL/WV3P9DON8vuP/hwwQLMCgYEAzyiv
rXx/ERnEQOpXEvPtCj04QUGCyW2o4nYIrN3OfO3PM8PnNfB8Z2LKDyuzX9qvWghg
z81KSNByW4OLdvrnX5natfxdu/6qAl18N+z/DFXuTgXi8nG9/2R5c3Gn5CQ5yab7
S4G4Pt7YwfoiqFv9j2b0DE3e5UVicGQIuWmaQ/UCgYBf61swraUXRsJ54YYU++xY
Gt/vbgCSacj1hnSebYsDqDB22tD7G5R9ubwfYe0mjf4H3XPekbdyKgdhVZTJdXww
7yEY/9lyAT0onbZsRliyCqHDzaqu/QHlrYOljdZUmrOSN/Dy5Y5VEPZEjLJqJjBf
/5HDNc0kzCWzQfzWui7xMQKBgGNA8ysEAz2GQul3XdDO3juRqWpaoPcxe0FFnFJ2
04A30JbUveqyFmjShE1QetjqRim06e2mRnksph4CoMeY31KGvKuFBsQT+BC6CdIh
0vFuGod3eoz+wjGjSi1tvysn0Cg1wSEkPcqhqukFl6VirdIPWc6rYKgo3klLJILx
feAhAoGABC0apuKQD2IZZXZtDuUI9I4AemPPh0yKvFfTJxmxQ0fTlWjqdcG5nYdh
tSMBlZwsd6DRlK7dWJ/WHZXuXNeOX6ehSQFmql5/XPNd7INa5My6DDPZr1chh0WJ
QgK94NXJDoDd1OZjpUBMPLVa8d20/RdGNW8OMolJpzEPhg0r7Ac=
-----END RSA PRIVATE KEY-----`

const SERVICE_MANAGER_SYSTEMD = "systemd"

type TestEnvironment struct {
	cmdRunner        boshsys.CmdRunner
	currentDeviceNum int
	writerPrinter    writerPrinter
	deviceMap        map[int]string
	sshClient        *ssh.Client
	AgentClient      *integrationagentclient.IntegrationAgentClient
	AgentSettings    boshsettings.Settings
	mbusUser         string
	mbusPass         string
	mbusPort         int
	serviceManager   string
	configDirty      bool
}

type writerPrinter interface {
	io.Writer

	Print(a ...interface{})
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}

func NewTestEnvironment(cmdRunner boshsys.CmdRunner, wp writerPrinter) (*TestEnvironment, error) {
	client, err := dialSSHClient(cmdRunner)
	if err != nil {
		return nil, err
	}

	const (
		mbusUser = "mbus-user"
		mbusPass = "mbus-pass"
		mbusPort = 6868
	)

	lgr := logger.NewWriterLogger(logger.LevelDebug, wp)
	mbusAddr := fmt.Sprintf("127.0.0.1:%d", mbusPort)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return client.Dial("tcp", mbusAddr)
		},
	}
	agentClient := integrationagentclient.NewIntegrationAgentClient(
		fmt.Sprintf("https://%s:%s@localhost:%d", mbusUser, mbusPass, mbusPort),
		"fake-director-uuid",
		1*time.Second,
		10,
		httpclient.NewHTTPClient(&http.Client{Transport: transport}, lgr),
		lgr,
	)

	return &TestEnvironment{
		cmdRunner:        cmdRunner,
		currentDeviceNum: 2,
		writerPrinter:    wp,
		deviceMap:        make(map[int]string),
		sshClient:        client,
		AgentClient:      agentClient,
		mbusUser:         mbusUser,
		mbusPass:         mbusPass,
		mbusPort:         mbusPort,
		// Start dirty so the first RestoreCleanBaseline (from SynchronizedBeforeSuite) writes the
		// default agent.json once. Thereafter only specs that call UpdateAgentConfig re-dirty it.
		configDirty: true,
	}, nil
}

// runBatch ships a whole deterministic command sequence to the VM in a single SSH round-trip.
// The script is fed to `sudo bash` on stdin via a quoted heredoc so it can contain single quotes,
// pipes, awk, etc. without any host-side escaping, and runs under `bash -euo pipefail` so a failing
// step surfaces instead of silently continuing mid-batch (add `|| true` for steps allowed to fail).
func (t *TestEnvironment) runBatch(script string) (string, error) {
	return t.RunCommand("sudo bash -euo pipefail <<'BOSH_BATCH_EOF'\n" + script + "\nBOSH_BATCH_EOF")
}

// detachMountFunc is a bash function (shared by DetachDevice and CleanupDataDir) that tears down a
// single mount directory: stop rsyslog first when detaching /var/log on systemd (see the long note
// below), then for each mount under $d -- deepest first -- fuser-kill holders and unmount (lazily for
// /var/log), and finally remove the directory. It expects $SM (the service manager) to be set by the
// caller.
//
// On systemd-based stemcells (e.g. resolute), rsyslog is intentionally left running for the whole
// test run so it can keep forwarding journald output into /var/log/bosh-agent.log, which
// /var/vcap/bosh/log/current is symlinked to. We stop it before the fuser-kill/unmount so it cleanly
// releases its open handles on /var/log instead of racing the lazy unmount. We deliberately do NOT
// restart it: /var/log is about to become a plain directory on the root disk again, not the
// /var/vcap/data/root_log bind mount rsyslog expects. The agent's own Bootstrap (SetupLogDir then
// SetupLoggingAndAuditing, see agent/bootstrap.go) re-establishes that bind mount and restarts
// logging, in that order, the next time StartAgent() runs.
//
// /var/log is unmounted lazily to prevent intermittent test failures: as of 2024-06-24 it is a bind
// mount of /var/vcap/data/root_log, and for reasons we don't fully understand `fuser -k` doesn't
// consistently terminate its holders in time for a plain umount. Because we later unmount
// /var/vcap/data, a lingering /var/log reference will eventually fail loudly there, so this is safe.
const detachMountFunc = `detach_mount() {
  d="$1"
  if [ "$d" = /var/log ] && [ "$SM" = systemd ]; then
    sudo systemctl stop syslog.socket rsyslog.service || true
  fi
  mps=$(sudo mount | grep "on $d" | cut -d' ' -f3 | awk '{ print length"\t"$0 }' | sort -rn | cut -f2- || true)
  for mp in $mps; do
    [ -n "$mp" ] || continue
    # fuser -km can kill sshd holders (wtmp/lastlog on /var/log); the trap '' HUP in the caller keeps
    # this reparented shell alive to finish the umount/rm. Its non-zero exit on empty mounts is expected.
    sudo fuser -km "$mp" || true
    if [ "$mp" = /var/log ]; then
      sudo umount --lazy "$mp" || true
    else
      sudo umount "$mp" || true
    fi
  done
  sudo rm -rf "$d"
}`

func (t *TestEnvironment) DetachDevice(dir string) error {
	if strings.HasPrefix(dir, "/dev/") {
		paths := []string{dir}
		for i := 1; i <= 3; i++ {
			paths = append(paths, fmt.Sprintf("%s%d", dir, i))
		}
		if _, err := t.RunCommand("sudo rm -f " + strings.Join(paths, " ")); err != nil {
			t.writerPrinter.Printf("DetachDevice: %s", err)
		}
		return nil
	}

	script := fmt.Sprintf("trap '' HUP\nSM=%s\n%s\ndetach_mount %s\n", t.serviceManager, detachMountFunc, dir)
	_, err := t.runBatch(script)
	return err
}

// RestoreCleanBaseline returns the VM to the baseline every spec expects: agent stopped, default
// agent.json, empty data dir, truncated logs, and no ephemeral loop devices. It is the single
// definition of "clean" shared by SynchronizedBeforeSuite (the first spec) and the suite AfterEach
// (every subsequent spec), so the two paths can't drift apart.
//
// The default agent.json is only rewritten when a spec changed it (tracked by configDirty). Only 3 of
// 49 specs override it, so the other 46 skip the rm+scp+mv restore; the env starts dirty so the first
// call (from SynchronizedBeforeSuite) writes the default once.
//
// StopAgent runs first on purpose: otherwise CleanupDataDir's `fuser -km /var/vcap/data` kills the
// running agent, systemd restarts it, and the restart races the umount/rm of /var/vcap/data (and
// crash-loops on the /var/log CleanupDataDir has already removed). DetectServiceManager must have run
// before this, since CleanupLogFile and GetSettingsFile branch on the service manager.
func (t *TestEnvironment) RestoreCleanBaseline() error {
	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	record(t.StopAgent())
	if t.configDirty {
		if err := t.UpdateAgentConfig(t.GetSettingsFile("")); err != nil {
			record(err)
		} else {
			t.configDirty = false
		}
	}
	record(t.CleanupDataDir())
	record(t.CleanupLogFile())
	record(t.ResetDeviceMap())

	return firstErr
}

// CleanupDataDir returns the VM's ephemeral state to baseline in a single SSH round-trip: stop monit
// (waiting until every process is unmonitored), unmount /tmp, tear down each ephemeral mount with the
// shared detach_mount helper, then recreate the emptied directories with their expected ownership and
// modes. The batch sets an empty HUP handler:
//
//	trap '' HUP
//
// so it runs to completion even if a fuser-kill inside detach_mount takes down the SSH session (see
// detachMountFunc). The monit-summary wait polls up to 3 times a second apart; "stopped" means every
// "Process" line reads "not monitored" (an empty summary trivially satisfies that).
//
// It also sweeps transient top-level uploads (logs.tgz, records.json, dummy_package.tgz,
// compiled-*.tgz) out of the blobstore assets dir so specs are isolated. `-maxdepth 1 -type f` leaves
// the read-only release/ fixtures (a subdirectory) intact, and the fake-blobstore binary and .ssh/
// live outside the assets dir so the sweep can't touch them. Fixtures staged by a spec's BeforeEach
// are re-copied each run, so sweeping them is safe.
func (t *TestEnvironment) CleanupDataDir() error {
	script := fmt.Sprintf(`trap '' HUP
SM=%s
%s
sudo /var/vcap/bosh/bin/monit stop all || true
stopped=0
for attempt in 1 2 3; do
  summary=$(sudo /var/vcap/bosh/bin/monit summary | grep 'Process' || true)
  total=$(printf '%%s\n' "$summary" | grep -c 'Process' || true)
  notmon=$(printf '%%s\n' "$summary" | grep -c 'not monitored' || true)
  if [ "$total" = "$notmon" ]; then stopped=1; break; fi
  sleep 1
done
if [ "$stopped" != 1 ]; then echo "ensureMonitStopped: monit processes not stopped" >&2; exit 1; fi
! mount | grep -q ' on /tmp ' || sudo umount /tmp
detach_mount /var/tmp
sudo rm -f /var/log/bosh-agent.log
detach_mount /var/log
detach_mount /opt
detach_mount /var/opt
detach_mount /var/vcap/data
sudo mkdir -p /var/tmp
sudo chmod 700 /var/tmp
sudo chmod 1777 /tmp
sudo mkdir -p /var/log
sudo chmod 775 /var/log
sudo chown root:syslog /var/log
sudo mkdir -p /var/opt
sudo chmod 775 /var/opt
sudo chown root:root /var/opt
sudo mkdir -p /opt
sudo chmod 775 /opt
sudo chown root:root /opt
if [ -d %[3]s ]; then
  find %[3]s -maxdepth 1 -type f -delete
fi
`, t.serviceManager, detachMountFunc, blobstoreDir)

	_, err := t.runBatch(script)
	return err
}

// ResetDeviceMap detaches every leftover loop device and removes its backing file in a single SSH
// round-trip. Noble's loop-device semantics require care:
//
//   - A plain `losetup -d` is unreliable: the /var/log bind mount and /var/vcap/data both sit on
//     these loop devices and are torn down with lazy unmounts, so a device can be transiently busy
//     and `losetup -d` returns "success" as an autoclear-pending detach that hasn't happened yet.
//   - Noble auto-scans partitions on loop devices, spawning child partition devices (loopNpM) whose
//     udev events must settle before the parent will detach.
//
// So we `udevadm settle` and retry (up to 5 attempts, sleeping 0.2s) until `losetup <dev>` shows the
// device gone, and only then delete backing files -- and only those no longer attached, since
// deleting the backing file of a still-attached loop is exactly what produced the "(deleted)" zombie
// loop devices we saw leaking across specs and destabilizing the next agent bootstrap.
func (t *TestEnvironment) ResetDeviceMap() error {
	script := `sudo udevadm settle
for dev in $(sudo losetup -a | cut -f1 -d:); do
  for attempt in 1 2 3 4 5; do
    sudo udevadm settle
    out=$(sudo losetup "$dev" 2>/dev/null || true)
    if [ -z "$out" ]; then break; fi
    sudo losetup -d "$dev" || true
    sleep 0.2
  done
  out=$(sudo losetup "$dev" 2>/dev/null || true)
  if [ -n "$out" ]; then echo "ResetDeviceMap: loop device $dev still attached after retries" >&2; fi
done
for f in /virtualfs-*; do
  [ -e "$f" ] || continue
  sudo losetup -ln -O BACK-FILE 2>/dev/null | awk -v f="$f" '$1==f{found=1} END{exit !found}' || sudo rm -f "$f"
done
`
	if _, err := t.runBatch(script); err != nil {
		return err
	}
	t.deviceMap = make(map[int]string)

	return nil
}

// CleanupLogFile truncates the agent log and, on systemd, rotates/vacuums the journal.
func (t *TestEnvironment) CleanupLogFile() error {
	script := "truncate -s 0 /var/vcap/bosh/log/current\n"

	if t.serviceManager == SERVICE_MANAGER_SYSTEMD {
		// NOTE: --vacuum-time=0s doesn't actually work
		script += "journalctl --flush --rotate --vacuum-time=1s\n"
	}

	_, err := t.runBatch(script)
	return err
}

func (t *TestEnvironment) CleanupSSH() error {
	_, err := t.RunCommand("sudo rm -rf /var/vcap/bosh_ssh")
	return err
}

func (t *TestEnvironment) LogFileContains(content string) bool {
	_, err := t.RunCommand(fmt.Sprintf(`sudo grep "%s" /var/vcap/bosh/log/current`, content))
	return err == nil
}

func (t *TestEnvironment) JournalContains(content string) bool {
	if t.serviceManager == SERVICE_MANAGER_SYSTEMD {
		_, err := t.RunCommand(fmt.Sprintf(`sudo journalctl -u bosh-agent.service | grep "%s"`, content))
		return err == nil
	}

	return false
}

func (t *TestEnvironment) EnsureRootDeviceIsLargeEnough() error {
	rootPartition, err := t.RunCommand("sudo findmnt -n -o source -T /")
	if err != nil {
		return err
	}
	rootDevice := rootPartition[:len(rootPartition)-1]

	output, err := t.RunCommand(fmt.Sprintf("sudo parted -m %s unit B print", rootDevice))
	if err != nil {
		return err
	}
	outputLines := strings.Split(strings.Trim(output, "\n"), "\n")[2:]
	rootPartitionFields := strings.Split(outputLines[0], ":")
	sizeInBytes, err := strconv.Atoi(strings.Trim(rootPartitionFields[2], "B"))
	if err != nil {
		return err
	}

	// Ensure we have enough space to create the fake loopback devices used in tests
	if sizeInBytes < 10000000000 {
		_, ignoredErr := t.RunCommand(fmt.Sprintf("sudo swapoff %s", rootDevice))
		if ignoredErr != nil {
			t.writerPrinter.Printf("EnsureRootDeviceIsLargeEnough: %s", ignoredErr)
		}

		for i := len(outputLines); i > 1; i-- {
			_, err = t.RunCommand(fmt.Sprintf("sudo parted %s rm %d", rootDevice, i))
			if err != nil {
				return err
			}
		}

		_, ignoredErr = t.RunCommand("sudo udevadm settle")
		if ignoredErr != nil {
			t.writerPrinter.Printf("EnsureRootDeviceIsLargeEnough: %s", ignoredErr)
		}
		_, err = t.RunCommand("cat /etc/lsb-release | grep -i jammy")
		// parteds behaviour changed and providing yes via params stopped working for jammy.
		// so test if we're running on jammy and adjust parted command
		if err != nil {
			_, err = t.RunCommand(fmt.Sprintf("sudo parted %s ---pretend-input-tty resizepart 1 yes 10000M", rootDevice))
			if err != nil {
				return err
			}
		} else {
			_, err = t.RunCommand(fmt.Sprintf("yes | sudo parted %s ---pretend-input-tty resizepart 1 10000M", rootDevice))
			if err != nil {
				return err
			}
		}
		_, err = t.RunCommand(fmt.Sprintf("sudo resize2fs -f %s", rootDevice))
		if err != nil {
			return err
		}
	}

	return nil
}

// attachDeviceBody loops over the partitions server-side so the whole AttachDevice sequence is one
// SSH round-trip. For each partition it grabs a free loop device, creates and attaches a backing
// file, reads the loop's major:minor straight from sysfs (/sys/block/<loop>/dev, decimal), and
// re-creates the block node at the expected path. It echoes one
// "MAP <deviceNum> <loop> <node> <major:minor>" line per partition for the Go wrapper to record.
// Parameters (DEV/NPARTS/SIZE/START) are prepended as bash assignments rather than fmt-interpolated
// so the body's ${devnum%%:*} parameter expansions don't collide with Go format verbs.
const attachDeviceBody = `n="$START"
for i in $(seq 0 "$NPARTS"); do
  if [ "$i" -eq 0 ]; then p="$DEV"; else p="${DEV}${i}"; fi
  sudo rm -rf "/virtualfs-$n"
  sudo truncate -s "${SIZE}M" "/virtualfs-$n"
  loop=$(sudo losetup --find --show "/virtualfs-$n")
  devnum=$(cat "/sys/block/$(basename "$loop")/dev")
  major=${devnum%%:*}
  minor=${devnum##*:}
  sudo rm -f "$p"
  sudo mknod "$p" b "$major" "$minor"
  echo "MAP $n $loop $p $major:$minor"
  n=$((n+1))
done
`

func (t *TestEnvironment) AttachDevice(devicePath string, partitionSize, numPartitions int) error {
	start := t.currentDeviceNum
	params := fmt.Sprintf("DEV=%s\nNPARTS=%d\nSIZE=%d\nSTART=%d\n", devicePath, numPartitions, partitionSize, start)

	out, err := t.runBatch(params + attachDeviceBody)
	if err != nil {
		t.writerPrinter.Printf("AttachDevice[%s]: %s\nOutput: %s", devicePath, err, out)
		return err
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 5 || fields[0] != "MAP" {
			continue
		}
		deviceNum, convErr := strconv.Atoi(fields[1])
		if convErr != nil {
			continue
		}
		t.deviceMap[deviceNum] = fields[2]
		t.writerPrinter.Printf("AttachDevice[%s]: loop=%s node=%s (b %s)\n", fields[3], fields[2], fields[3], fields[4])
	}
	t.currentDeviceNum = start + numPartitions + 1

	return nil
}

func (t *TestEnvironment) AttachPartitionedRootDevice(devicePath string, sizeInMB, rootPartitionSizeInMB int) (string, error) {
	err := t.AttachDevice(devicePath, sizeInMB, 3)
	if err != nil {
		return "", err
	}

	// Create only first partition, agent will partition the rest for ephemeral disk
	_, err = t.RunCommand(fmt.Sprintf("echo '1,%d,L,' | sudo sfdisk -uS %s", rootPartitionSizeInMB*2048, devicePath))
	if err != nil {
		return "", err
	}

	rootLink, err := t.RunCommand("df / | grep /dev/ | cut -d' ' -f1")
	if err != nil {
		return "", err
	}

	oldRootDevice, err := t.RunCommand(fmt.Sprintf("readlink -f %s", rootLink))
	if err != nil {
		return "", err
	}

	_, err = t.RunCommand(fmt.Sprintf("sudo mv %s %s-temp", strings.TrimSpace(oldRootDevice), strings.TrimSpace(oldRootDevice)))
	if err != nil {
		return "", err
	}

	// Agent reads the symlink to get root device
	// Create a symlink to our fake device
	_, err = t.RunCommand(fmt.Sprintf("sudo ln -sf %s1 %s", devicePath, strings.TrimSpace(rootLink)))

	if err != nil {
		return strings.TrimSpace(oldRootDevice), err
	}

	// Let udev finish processing the sfdisk partition-table change and the /dev symlink swap above
	// before the agent boots and resolves the root device. Without settling, the agent can read the
	// root partition before its state is stable and bootstrap fails with "Cannot get filesystem type
	// for root file system".
	_, err = t.RunCommand("sudo udevadm settle")
	if err != nil {
		return strings.TrimSpace(oldRootDevice), err
	}

	return strings.TrimSpace(oldRootDevice), nil
}

func (t *TestEnvironment) DetachPartitionedRootDevice(rootLink string, devicePath string) (err error) {
	// Restore the real root device node no matter what happens below. During the test rootLink (e.g.
	// /dev/sdb2) is a symlink to the fake loop partition; if any loop-device teardown step below failed
	// and we returned early without the mv, /dev/sdb2 would be left removed/dangling and EVERY
	// subsequent spec's agent would fail bootstrap with "Cannot get filesystem type for root file
	// system" -- turning one flaky spec into a whole-suite cascade (suite timeout). Deferring the
	// restore keeps a single spec's failure contained to that spec.
	defer func() {
		if _, restoreErr := t.RunCommand(fmt.Sprintf("sudo mv %s-temp %s", rootLink, rootLink)); restoreErr != nil && err == nil {
			err = restoreErr
		}
	}()

	if _, err = t.RunCommand(fmt.Sprintf("sudo rm -f %s", rootLink)); err != nil {
		return err
	}

	partitionPath := devicePath
	for i := 3; i >= 0; i-- {
		if i > 0 {
			partitionPath = fmt.Sprintf("%s%d", devicePath, i)
		}

		if _, losetupErr := t.RunCommand(fmt.Sprintf("losetup %s", partitionPath)); losetupErr == nil {
			if output, _ := t.RunCommand(fmt.Sprintf("sudo mount | grep '%s ' | awk '{print $3}'", partitionPath)); output != "" { //nolint:errcheck
				for _, path := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
					_, ignoredErr := t.RunCommand(fmt.Sprintf("sudo umount -l %s", path))
					if ignoredErr != nil {
						t.writerPrinter.Printf("DetachPartitionedRootDevice: %s", ignoredErr)
					}

				}

			}

			if i > 0 {
				_, _ = t.RunCommand(fmt.Sprintf("sudo parted %s rm %d", devicePath, i)) //nolint:errcheck
			}

			// Settle the udev events from the parted change above before touching the loop device.
			_, _ = t.RunCommand("sudo udevadm settle") //nolint:errcheck

			if ignoredErr := t.DetachLoopDevice(partitionPath); ignoredErr != nil {
				t.writerPrinter.Printf("DetachPartitionedRootDevice: deferring detach of %s to ResetDeviceMap: %s", partitionPath, ignoredErr)
			}

			if err = t.RemoveDevice(partitionPath); err != nil {
				return err
			}
		}

	}

	return nil
}

func (t *TestEnvironment) RemoveDevice(devicePath string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo rm -f %s", devicePath))
	return err
}

func (t *TestEnvironment) DetachLoopDevice(devicePath string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo losetup -d %s", devicePath))
	return err
}

func (t *TestEnvironment) DetachLoopDevices() error {
	_, err := t.RunCommand("sudo losetup -D")
	return err
}

func (t *TestEnvironment) SetUpDummyNetworkInterface(ip, mac string) error {
	return t.RunCommandChain(
		"sudo ip link add dummy0 type dummy",
		fmt.Sprintf("sudo ip link set dev dummy0 address %s", mac),
		"sudo ip link set dev dummy0 arp on",
		fmt.Sprintf("sudo ip addr add %s dev dummy0", ip),
		fmt.Sprintf("sudo ip neigh add to %s lladdr %s dev dummy0 nud reachable", ip, mac),
	)
}

func (t *TestEnvironment) RunCommandChain(commands ...string) error {
	for _, command := range commands {
		_, err := t.RunCommand(command)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TestEnvironment) TearDownDummyNetworkInterface() error {
	_, err := t.RunCommand("sudo rmmod dummy || true")
	return err
}

func (t *TestEnvironment) UpdateAgentConfig(configFile string) error {
	// Any write to agent.json marks it dirty so RestoreCleanBaseline knows to restore the default; the
	// 46 specs that never touch it skip that restore.
	t.configDirty = true
	_, err := t.RunCommand("sudo rm -f /var/vcap/bosh/agent.json")
	if err != nil {
		return err
	}
	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), configFile), "/var/vcap/bosh/agent.json")
}

// CopyFileToPath streams the local file to its destination over the persistent SSH tunnel by piping
// the bytes into `sudo tee` via the session's stdin, reusing the connection instead of a fresh scp.
// Streaming via stdin (not argv) keeps it binary-safe and free of ARG_MAX and shell-quoting limits,
// so it works for arbitrarily large blobs too.
func (t *TestEnvironment) CopyFileToPath(localPath string, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	s, err := t.sshClient.NewSession()
	if err != nil {
		return errors.WrapError(err, "Unable to establish SSH session: ")
	}
	defer s.Close() //nolint:errcheck

	s.Stdin = f
	t.writerPrinter.Printf("Remote Cmd Runner Streaming %s to remote %s\n", localPath, remotePath)
	// SSH runs this through the remote shell, so remotePath must be shell-quoted rather than
	// Go-quoted (%q leaves $ and backticks live inside double quotes, allowing command substitution).
	// Single quotes suppress all shell interpretation; embedded single quotes are escaped as '\''.
	// `tee --` marks the end of options so a path can never be read as a flag.
	quotedPath := "'" + strings.ReplaceAll(remotePath, "'", `'\''`) + "'"
	if out, err := s.CombinedOutput(fmt.Sprintf("sudo tee -- %s > /dev/null", quotedPath)); err != nil {
		return errors.WrapErrorf(err, "Error streaming to %s: %s", remotePath, string(out))
	}
	return nil
}

func (t *TestEnvironment) RestartAgent() error {
	err := t.StopAgent()
	if err != nil {
		return err
	}

	return t.StartAgent()
}

func (t *TestEnvironment) StopAgent() error {
	waitSeconds := 30
	if raw := os.Getenv("BOSH_AGENT_SV_STOP_TIMEOUT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("BOSH_AGENT_SV_STOP_TIMEOUT must be a positive integer, got %q", raw)
		}
		waitSeconds = parsed
	}
	var err error
	if t.serviceManager == SERVICE_MANAGER_SYSTEMD {
		_, err = t.RunCommand("sudo systemctl stop agent")
	} else {
		_, err = t.RunCommand(fmt.Sprintf("sudo sv -w %d stop agent", waitSeconds))
	}

	return err
}

func (t *TestEnvironment) StartAgent() error {
	var err error
	if t.serviceManager == SERVICE_MANAGER_SYSTEMD {
		// Clear any accumulated systemd start-limit / failed state before (re)starting. CleanupDataDir's
		// `fuser -km /var/log` SIGKILLs auditd (its log lives under /var/log/audit); systemd then restarts
		// it in a tight loop until it trips its StartLimitBurst (5 starts / 10s) and refuses to start it at
		// all. A StopAgent -> CleanupDataDir -> StartAgent sequence can land inside that window, so the
		// agent's bootstrap fails to start auditd and crash-loops. reset-failed clears the counters so
		// auditd (and the agent) can start cleanly; it is a harmless no-op when nothing has failed.
		_, err = t.RunCommand("sudo systemctl reset-failed auditd.service rsyslog.service bosh-agent.service 2>/dev/null || true")
		if err != nil {
			return err
		}
		// systemctl start agent will return immediately, but agent takes a moment to bootstrap and mount /var/log.
		// If we start rsyslog.service now, it will block in ExecStartPre waiting for the agent to mount /var/log.
		// Since we run both asynchronously via systemd, this correctly simulates the system boot dependency.
		_, err = t.RunCommand("sudo systemctl start rsyslog.service --no-block")
		if err != nil {
			return err
		}
		_, err = t.RunCommand("sudo systemctl start agent")
		if err != nil {
			return err
		}

		// Wait for the agent to bootstrap far enough to mount /var/log, which unblocks rsyslog's
		// ExecStartPre (wait_for_var_log_to_be_mounted) so rsyslog starts and creates
		// /var/log/bosh-agent.log on the first bosh-agent-tagged message.
		waitCmd := `i=0; while [ "$i" -lt 300 ]; do sudo test -e /var/log/bosh-agent.log && break; ` +
			`i=$((i+1)); sleep 0.2; done; ` +
			`if sudo test -e /var/log/bosh-agent.log; then echo "bosh-agent.log present after ${i} polls (~$((i/5))s)"; ` +
			`else echo "bosh-agent.log ABSENT after ${i} polls"; fi`
		out, err := t.RunCommand(waitCmd)
		t.writerPrinter.Printf("StartAgent: %s\n", strings.TrimSpace(out))
		if err != nil {
			return err
		}
		if !strings.Contains(out, "present") {
			return fmt.Errorf("StartAgent: %s", strings.TrimSpace(out))
		}
	} else {
		_, err = t.RunCommand("nohup sudo sv start agent &")
	}

	return err
}

func (t *TestEnvironment) DumpDiagnostics() {
	diagnosticCommands := []string{
		"sudo systemctl status bosh-agent.service --no-pager --full || true",
		"sudo systemctl show bosh-agent.service -p NRestarts -p ExecMainStartTimestamp -p ActiveEnterTimestamp || true",
		"sudo systemctl status rsyslog.service --no-pager --full || true",
		"sudo systemctl status auditd.service --no-pager --full || true",
		"sudo systemctl show auditd.service -p NRestarts -p StartLimitBurst -p StartLimitIntervalUSec || true",
		`sudo journalctl -u auditd.service -o short-precise --no-pager | tail -n 60 || true`,
		`sudo journalctl -u bosh-agent.service -o short-precise --no-pager | grep -iE "error|fail|bootstrap|ephemeral|partition|panic|Starting|Stopped|Deactivated|Main process|scheduled restart" | head -n 120 || true`,
		"mountpoint /var/log || true",
		`mount | grep -E "/var/log|/var/vcap/data" || true`,
		"sudo losetup -a || true",
		"ls -al /dev/sd* /dev/loop* 2>/dev/null || true",
		"df / | grep /dev/ || true",
		"sudo ls -al /var/log/bosh-agent.log || true",
		"sudo ls -al /var/vcap/data/root_log 2>/dev/null || true",
	}

	t.writerPrinter.Println("=========== BEGIN DIAGNOSTICS DUMP (spec failed) ===========")
	for _, command := range diagnosticCommands {
		out, err := t.RunCommand(command)
		t.writerPrinter.Printf("\n$ %s\n%s\n(exit err: %v)\n", command, out, err)
	}
	t.writerPrinter.Println("=========== END DIAGNOSTICS DUMP ===========")
}

func (t *TestEnvironment) DetectServiceManager() error {
	out, err := t.RunCommand(fmt.Sprintf("if [ -d /etc/service/agent/ ] >/dev/null 2>&1; then echo sv; else echo %s; fi", SERVICE_MANAGER_SYSTEMD))
	if err != nil {
		return err
	}

	if result := strings.TrimSpace(out); result == SERVICE_MANAGER_SYSTEMD {
		t.serviceManager = result
	}

	return nil
}

func (t *TestEnvironment) GetServiceManager() string {
	return t.serviceManager
}

func (t *TestEnvironment) GetSettingsFile(specification string) string {
	suffix := ""

	if specification != "" {
		suffix += "-"
		suffix += specification
	}

	if sm := t.GetServiceManager(); sm != "" {
		suffix += "-"
		suffix += sm
	}

	return fmt.Sprintf("file-settings-agent%s.json", suffix)
}

func (t *TestEnvironment) WaitForAgent() error {
	var err error
	for i := 1; i < 90; i++ {
		t.writerPrinter.Printf("Trying to contact agent via ssh tunnel...")
		time.Sleep(1 * time.Second)
		_, err = t.AgentClient.Ping()
		if err == nil {
			return nil
		}
	}
	t.writerPrinter.Printf("WaitForAgent %s", err.Error())
	return err
}

func (t *TestEnvironment) StartBlobstore() error {
	_, ignoredErr := t.RunCommand("sudo killall -9 fake-blobstore")
	if ignoredErr != nil {
		t.writerPrinter.Printf("StartBlobstore: %s", ignoredErr)
	}

	_, err :=
		t.RunCommand(fmt.Sprintf("nohup /home/agent_test_user/fake-blobstore -host 127.0.0.1 -port 9091 -assets %s &> /dev/null &", blobstoreDir))

	return err
}

func (t *TestEnvironment) CreateSettingsFile(settings boshsettings.Settings) error {
	emptyCert := boshsettings.CertKeyPair{}
	if settings.Env.Bosh.Mbus.Cert == emptyCert {
		settings.Env.Bosh.Mbus.Cert.Certificate = agentCert
		settings.Env.Bosh.Mbus.Cert.PrivateKey = agentKey
	}
	if settings.AgentID == "" {
		settings.AgentID = "fake-agent-id"
	}
	if settings.Mbus == "" {
		settings.Mbus = "https://mbus-user:mbus-pass@127.0.0.1:6868"
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(t.AssetsDir(), "test.json"), settingsJSON, 0644)
	if err != nil {
		return err
	}
	// Remove any stale settings files in one round-trip; this runs in every spec's BeforeEach.
	_, err = t.runBatch("rm -f /var/vcap/settings.json /var/vcap/bosh/settings.json /var/vcap/bosh/update_settings.json")
	if err != nil {
		return err
	}

	err = t.CopyFileToPath(filepath.Join(t.AssetsDir(), "test.json"), "/var/vcap/settings.json")
	if err != nil {
		return err
	}
	return err
}

func (t *TestEnvironment) GetVMNetworks() (boshsettings.Networks, error) {
	return boshsettings.Networks{
		"eth0": {
			Type: "dynamic",
		},
	}, nil
}

func (t *TestEnvironment) GetFileContents(filePath string) (string, error) {
	return t.RunCommand(fmt.Sprintf("sudo cat %s", filePath))
}

func (t *TestEnvironment) RunCommand(command string) (string, error) {
	s, err := t.sshClient.NewSession()

	if err != nil {
		t.writerPrinter.Println("Remote Cmd Runner", "NewSession() FAILED TO EXECUTE: %s ERROR: %s\n", command, err)
		return "", errors.WrapError(err, "Unable to establish SSH session: ")
	}
	defer s.Close() //nolint:errcheck
	t.writerPrinter.Println("Remote Cmd Runner", "Running remote command '%s'", command)
	out, err := s.CombinedOutput(command)
	if err != nil {
		t.writerPrinter.Println("CombinedOutput() FAILED TO EXECUTE: %s ERROR: %s", command, err)
		return string(out), errors.WrapErrorf(err, "Error running %s", command)
	}
	return string(out), nil
}

func (t *TestEnvironment) CreateSensitiveBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/sensitive_blobs")
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf("/var/vcap/data/sensitive_blobs/%s", blobID))
}

func (t *TestEnvironment) CreateBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/blobs")
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf("/var/vcap/data/blobs/%s", blobID))
}

func (t *TestEnvironment) CreateBlobFromAssetInActualBlobstore(assetPath, blobstorePath, blobID string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo mkdir -p %s", blobstorePath))
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf(blobstorePath, blobID))
}

func (t *TestEnvironment) agentDir() string {
	integrationPath, _ := os.Getwd() //nolint:errcheck
	agentDir, _ := filepath.Split(integrationPath)
	return agentDir
}

func (t *TestEnvironment) AssetsDir() string {
	return filepath.Join(t.agentDir(), "integration", "assets")
}

// blobstoreDir is the on-VM directory backing the fake blobstore. BlobstoreDir, StartBlobstore, and
// CleanupDataDir all target it, so they share this constant to stay in sync.
const blobstoreDir = "/home/agent_test_user/blobstore"

func (t *TestEnvironment) BlobstoreDir() string {
	return blobstoreDir
}

func dialSSHClient(cmdRunner boshsys.CmdRunner) (*ssh.Client, error) {
	stdout, _, _, err := cmdRunner.RunCommand("cat", "ssh-config")
	if err != nil {
		return nil, err
	}
	config, err := ssh_config.Decode(strings.NewReader(stdout))
	if err != nil {
		return nil, err
	}
	user, err := config.Get("agent_vm", "User")
	if err != nil {
		return nil, err
	}
	addr, err := config.Get("agent_vm", "HostName")
	if err != nil {
		return nil, err
	}
	port, err := config.Get("agent_vm", "Port")
	if err != nil {
		return nil, err
	}
	keyPath, err := config.Get("agent_vm", "IdentityFile")
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	testVMAddress := fmt.Sprintf("%s:%s", addr, port)
	testVMSSHConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	jumpboxAddr, err := config.Get("jumpbox", "HostName")
	if err != nil {
		return nil, err
	}
	if jumpboxAddr != "" {
		jumpboxUser, err := config.Get("jumpbox", "User")
		if err != nil {
			return nil, err
		}
		jumpboxKeyPath, err := config.Get("jumpbox", "IdentityFile")
		if err != nil {
			return nil, err
		}
		jumpboxKey, err := os.ReadFile(jumpboxKeyPath)
		if err != nil {
			return nil, err
		}
		jumpboxSigner, err := ssh.ParsePrivateKey(jumpboxKey)
		if err != nil {
			return nil, err
		}

		jumpboxClient, err :=
			ssh.Dial("tcp", fmt.Sprintf("%s:%s", jumpboxAddr, "22"), &ssh.ClientConfig{
				User: jumpboxUser,
				Auth: []ssh.AuthMethod{
					ssh.PublicKeys(jumpboxSigner),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			})
		if err != nil {
			return nil, err
		}

		proxyConnection, err := jumpboxClient.Dial("tcp", testVMAddress)
		if err != nil {
			return nil, err
		}

		proxyClientConnection, proxyClientChannel, proxyClientRequest, err :=
			ssh.NewClientConn(proxyConnection, testVMAddress, testVMSSHConfig)
		if err != nil {
			return nil, err
		}

		return ssh.NewClient(proxyClientConnection, proxyClientChannel, proxyClientRequest), nil
	}
	return ssh.Dial("tcp", testVMAddress, testVMSSHConfig)
}
