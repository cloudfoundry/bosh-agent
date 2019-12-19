package compiler

import (
 boshsys "github.com/cloudfoundry/bosh-utils/system"
 )

// this is a decorator around a bosh utils filesystem instance,
// that retries at least rename operations

//newFS (takes an fs)

type RetryingFs struct {
	underlyingFs boshsys.FileSystem

}

func (fs RetryingFs) Stat() {
	fs.underlyingFs.Stat("mycoolfile")
}


// func rename () {
//   wrap retry logic {
//     super.rename
//   }
// }
