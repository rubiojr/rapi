# Recommended Reading

## General

* restic's design document: https://restic.readthedocs.io/en/latest/100_references.html#design

* Some design questions: https://github.com/restic/restic/issues/347

* Using restic to backup encrypted containers?: https://forum.restic.net/t/using-restic-to-backup-encrypted-containers/408/4

* Some backends affect restic's performance, in particular when they don't support ranged reads: https://forum.restic.net/t/huge-amount-of-data-read-from-s3-backend/2321/6


## Security/Crypto

* Filippo Valsorda's article about restic's cryptography: https://blog.filippo.io/restic-cryptography/

* Repository key security: https://forum.restic.net/t/is-storing-key-in-the-backup-location-really-safe/2021/2

## Chunking

* Foundation - Introducing Content Defined Chunking: https://restic.net/blog/2015-09-12/restic-foundation1-cdc/

## Pack files

* Control the minimal pack files size: https://forum.restic.net/t/control-the-minimal-pack-files-size/617

* How to set the Min/Max Pack Size?:https://forum.restic.net/t/how-to-set-the-min-max-pack-size/1268/3

## Blobs

* Restoring file blobs out of order: https://github.com/restic/restic/pull/2195

## Troubleshooting

* Corrupt pack files: https://github.com/restic/restic/issues/2191
