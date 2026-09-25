# Approved disposable service volume

This module retains the approved NYC3 VM, **320 GiB** service volume and cost
scope. It leaves the new volume unformatted. The provider's volume size is
specified in GiB; no implicit decimal conversion or larger allocation is
needed. [DigitalOcean Terraform volume resource](https://registry.terraform.io/providers/digitalocean/digitalocean/latest/docs/resources/volume)

Only the root operator invokes `infra/demo/scripts/cloud_volume.py`. The helper
does not allocate cloud resources, discover a replacement disk, resize storage
or grant sudo to application services. Existing devices and filesystems are
preserved. The exact Terraform output `service_volume_bootstrap` binds the
deployment, volume ID/name, droplet, approved quote hash, NYC3 region, size and
fixed mount. Retain it in the private operator vault with the Terraform state.

After an approved **fresh creation**, export that JSON to a private file and
install it root-owned, mode 0600 on the intended droplet. Verify the provider's
attachment matches its volume and droplet IDs before invoking the helper.
The explicit `--new-volume-id` assertion means the operator has established
that this resource was just created for this deployment. Signature absence
alone cannot prove that arbitrary raw data is disposable. Do not use this
operation on an imported, previously attached or unidentified volume.

```bash
# On the operator machine, after applying the reviewed exact plan:
umask 077
terraform output -json service_volume_bootstrap > cloud-volume.json
sha256sum cloud-volume.json
# Transfer the exact bytes to /etc/feam/cloud-volume.json, owner root, mode0600.

# On the approved droplet, substitute the reviewed hash and fresh volume UUID:
sudo python3 cloud_volume.py inspect --manifest /etc/feam/cloud-volume.json \
  --confirm-sha256 REVIEWED_FILE_SHA256
sudo python3 cloud_volume.py initialize --manifest /etc/feam/cloud-volume.json \
  --confirm-sha256 REVIEWED_FILE_SHA256 --new-volume-id EXACT_NEW_VOLUME_UUID
```

`initialize` requires the exact stable provider device name, a writable whole
320 GiB disk, no partitions, mounts, device holders or swap, and no `wipefs`
or `blkid` signature. The provider's `/dev/disk/by-id/scsi-0DO_Volume_<name>`
link is stable across boot; `/dev/sdX` is not used as configured authority.
[DigitalOcean volume naming](https://docs.digitalocean.com/products/volumes/details/naming-conventions/)

It writes a durable root-only ownership intent before invoking `mkfs.ext4`
with 4 KiB blocks, 65,536 bytes per inode, 256-byte inodes, zero reserved blocks,
a 256 MiB journal, no discard and fully initialized inode tables/journal.
There is no force-format flag. The deployment UUID becomes the filesystem UUID.
These settings apply only to the new service volume, not the OS or existing
Ubuntu filesystems. Ext4 normally reserves 5% of blocks and the inode ratio
cannot be changed after creation. That default would violate this volume's
full-pool free-space budget. [mke2fs documentation](https://man7.org/linux/man-pages/man8/mke2fs.8.html)

The fixed `home-feam\x2dservice\x2ddata.mount` unit mounts that UUID at
`/home/feam-service-data` with `nosuid,nodev,noexec,nodiscard`. It has no
`nofail` fallback. Unknown mount units or nonempty unmounted directories are
refused. Child loop mount units must use `RequiresMountsFor=/home/feam-service-data`;
the W1 helper must check this root's containing filesystem UUID, rather than
the containing filesystem of `/home`. Set Ansible `feam_parent_uuid` to the
bootstrap filesystem UUID on cloud hosts. No application starts until the
ordinary storage, runtime and service checks pass.

If initialization is interrupted, retain the device and ownership intent.
Never delete the intent to retry formatting. Once the exact existing
filesystem passes inspection, the explicit `mount` mode may install/start the
matching mount unit without formatting. A partial or blank interrupted format
requires operator investigation and a separately reviewed fresh resource;
this helper does not repair or replay it. `verify` checks ownership, geometry,
the exact mounted UUID and current free-space floor without formatting.

## Capacity evidence and remaining gate

On 2026-09-25, e2fsprogs 1.47.0 on the existing Ubuntu host formatted a newly
created sparse regular-file model. It had 320 GiB logical size and used
1,618,432,000 physical bytes for the bounded experiment; the temporary directory
was removed immediately. No loop device or filesystem mount was used.
The [sanitized receipt](../../../../docs/evaluations/web-demo/cloud-volume-format-model.json)
records 341,891,616,768 free bytes after formatting. After reserving the entire
269 GiB child pool, a conservative 48 GiB floor and an additional 1 GiB for
outer-filesystem metadata, 441,716,736 bytes remain. This is a format model,
not a mounted cloud-volume or ten-user capacity result.

The helper independently gates the real fresh mount using `statvfs`:

```text
available_bytes - 269 GiB >= max(20 GiB, ceil(filesystem_bytes * 0.15)) + 1 GiB
```

Keep all application artifacts inside their assigned child filesystems; do
not put backup archives or scratch data on the outer service filesystem.
After the full pool is physically allocated, run W1 storage verification and
this helper's `verify` mode again. A failed gate stops deployment within the
approved size; it does not increase the volume, reduce quotas or relax the
free-space floor. Actual cloud attachment, full allocation, mount ordering,
repeat converge, reboot and scoped teardown remain W9 acceptance work.

Local checks (no cloud access):

```bash
python3 -m unittest discover -s infra/demo/tests/w9 -p 'test_cloud_volume.py' -v
terraform -chdir=infra/demo/terraform/cloud-host fmt -check
terraform -chdir=infra/demo/terraform/cloud-host validate
terraform -chdir=infra/demo/terraform/cloud-host test
```
