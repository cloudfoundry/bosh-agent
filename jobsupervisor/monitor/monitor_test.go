package monitor

import (
	"fmt"
	"math"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cpuTestCases = []struct {
	prevKernel, prevUser, prevIdle uint64
	currKernel, currUser, currIdle uint64
	loadKernel, loadUser, loadIdle float64
}{
	{
		0, 0, 0,
		300, 500, 200,
		0.125, 0.625, 0.25,
	},
	{
		300, 500, 200,
		600, 1000, 400,
		0.125, 0.625, 0.25,
	},
	{
		300, 500, 200,
		300, 500, 100, // Regression
		0, 0, 0,
	},
}

func matchFloat(val, exp float64) error {
	if math.Abs(val-exp) < 0.000001 {
		return nil
	}
	return fmt.Errorf("Expected %.6f to Equal %.6f", val, exp)
}

var _ = Describe("CPU", func() {
	Context("when calculating CPU usage", func() {
		It("should correctly report SystemLevel User, Kernel and Idle", func() {
			for _, x := range cpuTestCases {
				m := Monitor{}
				m.kernel.previous = x.prevKernel
				m.user.previous = x.prevUser
				m.idle.previous = x.prevIdle
				m.calculateSystemCPU(x.currKernel, x.currUser, x.currIdle)
				Expect(matchFloat(m.kernel.load, x.loadKernel)).To(Succeed())
				Expect(matchFloat(m.user.load, x.loadUser)).To(Succeed())
				Expect(matchFloat(m.idle.load, x.loadIdle)).To(Succeed())
			}
		})

		It("should correctly report ProcessLevel User, Kernel and Idle", func() {
			var processCpuCases = []struct {
				deltaKernel, deltaUser, deltaIdle uint64
				loadKernel, loadUser              float64
				prevCpu                           uint64
				ticks                             uint64
				expLoad                           float64
			}{
				{
					300, 500, 200,
					0.125, 0.625,
					0,
					400,
					0.5,
				},
				{
					600, 1000, 400,
					0.125, 0.625,
					0,
					800,
					0.5,
				},
				{
					600, 1000, 400,
					0.125, 0.625,
					400,
					800,
					0.25,
				},
				// Zero case
				{
					100, 100, 200,
					0.125, 0.625,
					100,
					100,
					0.00,
				},
			}
			for _, x := range processCpuCases {
				p := &Process{
					name: "test",
					pid:  123,
				}
				p.cpu.previous = x.prevCpu
				m := Monitor{
					pids: make(map[uint32]*Process),
				}
				m.pids[p.pid] = p

				m.kernel.delta = x.deltaKernel
				m.user.delta = x.deltaUser
				m.idle.delta = x.deltaIdle

				m.kernel.load = x.loadKernel
				m.user.load = x.loadUser

				m.calculateProcessCPU(p, x.ticks)
				Expect(matchFloat(p.cpu.load, x.expLoad)).To(Succeed())
			}
		})
	})

	Context("when watching a processes", func() {
		It("should add the new process and creates a handle", func() {
			pid := uint32(syscall.Getpid())
			m, err := New(-1)
			Expect(err).To(BeNil())
			err = m.WatchProcess("foo", pid)
			Expect(err).To(BeNil())
			Expect(m.pids[pid]).ToNot(BeNil())
			Expect(m.pids[pid].handle).ToNot(BeNil())
		})

		It("should throw an err for invalid pid", func() {
			const pid uint32 = 1<<32 - 1
			m, err := New(-1)
			Expect(err).To(BeNil())
			err = m.WatchProcess("foo", pid)
			Expect(err).ToNot(BeNil())
		})

		It("should throw an err the process has died", func() {
			pid := uint32(syscall.Getpid())
			m, err := New(time.Millisecond * 50)
			Expect(err).To(BeNil())
			err = m.WatchProcess("foo", pid)
			Expect(err).To(BeNil())
			Expect(m.pids[pid]).ToNot(BeNil())
			Expect(m.pids[pid].handle).ToNot(BeNil())

			time.Sleep(time.Second * 2)
			m.pids[pid].handle = 0

			Eventually(func() bool {
				_, exists := m.getProc(pid)
				return !exists
			}).Should(Equal(true))
		})
	})
})
