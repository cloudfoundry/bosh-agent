# Windows root certificate baseline

The Windows agent restores its root certificate baseline before applying the
Director's trusted certificates. Provision that baseline **before starting this
agent version**, both for new stemcells and for upgrades. Missing baselines stop
certificate updates before the trust store changes.

The baseline is `<base-dir>\bosh\root-cert-backup\rootCertBackup.sst`. Its directory
and file must have protected DACLs that grant only Local SYSTEM (`S-1-5-18`) and
built-in Administrators (`S-1-5-32-544`) full control, with either principal as owner.
The directory's parent must already exist and allow only trusted principals to
write. Ancestors must not permit untrusted replacement or use reparse points.
The agent validates these properties each time; it never repairs and trusts an
existing insecure baseline.

For a new stemcell, export its known-clean LocalMachine Root store during image
construction, before running an older agent or installing deployment certificates.
Approve that SST's SHA256 through the image build process. For an existing VM,
recover the baseline from an independently verified image or backup. Do not
approve a digest merely by hashing the legacy temporary backup or the running
VM's current root store: either may already contain untrusted certificates.

Run the following reviewed source helper as SYSTEM (or an administrator with the
rights required to set SYSTEM ownership), using that independently approved SST
and digest. It reads and hashes the same bounded input, validates its certificate
contents, creates the destination with a private DACL atomically, and refuses to
overwrite an existing baseline. It does not modify any certificate store.

```powershell
$ErrorActionPreference = 'Stop'
. .\platform\cert\windows_cert_store.ps1
Install-RootCertificateBaseline `
    -BackupDirectory 'C:\var\vcap\bosh\root-cert-backup' `
    -BaselinePath 'C:\verified-image\root-cert-baseline.sst' `
    -ExpectedSha256 '<independently approved 64-character SHA256>'
```

Use the actual configured base directory. Review and correct the parent ACL or
rebuild the VM if validation rejects it; changing an untrusted file's ACL cannot
make its existing contents trustworthy. If the agent finds an insecure existing
destination, quarantine it explicitly before installing a verified baseline.

The old `<base-dir>\data\tmp\rootCertBackup.sst` must also be removed or quarantined
by an operator after the verified baseline is installed. The agent refuses to
consume it and does not migrate its contents. A missing legacy file does not
prove a clean installation, so the agent never automatically recaptures the
current root store. Retain the protected baseline across agent restarts/upgrades.
