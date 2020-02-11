# ZFS Pool Statistics

Scrapes metrics from the outputs of both `zpool status` and `zpool list`. The
metrics are described in comments at the top of the script. Written in Perl, it
should work with any Perl 5 from the past decade, or longer.

The format of the `status` subcommand, in particular, is not stable, and does
not have a machine-parseable display option. The plugin already accounts for
differences introduced in the [Sequential Scrubs and
Resilvers](https://github.com/zfsonlinux/zfs/pull/6256) feature, but future
changes may cause metrics to go missing.

The plugin has been tested on the following platforms, but should work on any
system running [OpenZFS](http://open-zfs.org/wiki/Main_Page):
* CentOS/RHEL 7 (ZoL 0.7, 0.8)
* FreeBSD 12.1
* SmartOS (various platform images from 2018-2019)
* Ubuntu 18.04 LTS (ZoL 0.7)
* Ubuntu 16.04 LTS (ZoL 0.6)

## Permissions on /dev/zfs

The agent usually runs as an unprivileged user. With ZFSonLinux prior to 0.7,
the permissions on the `/dev/zfs` device were restricted to root. In order for
the plugin to be able to run the `zpool` command as an unprivileged user, a
[workaround](https://github.com/zfsonlinux/zfs/issues/362#issuecomment-1987600)
must be used to set the necessary permissions on the device.

This is not an issue for ZFSonLinux 0.7.0 or later.

## A Note on Pool Sizes

Note that the sizes reported by `zpool list` for size/alloc/free may differ,
sometimes significantly, from those reported by `zfs list`. It includes space
devoted to things like RAID-Z parity and accounting reservations that are not
available to ZFS datasets. It does _not_ include things like ZFS dataset
reservations, which count as "used" to `zfs list` but may not be physically
allocated in the pool.

Per the `zpool` man page:

> The space usage properties report actual physical space available to the
> storage pool.  The physical space can be different from the total amount of
> space that any contained datasets can actually use.  The amount of space used
> in a raidz configuration depends on the characteristics of the data being
> written.  In addition, ZFS reserves some space for internal accounting that
> the zfs(8) command takes into account, but the zpool command does not.  For
> non-full pools of a reasonable size, these effects should be invisible.  For
> small pools, or pools that are close to being completely full, these
> discrepancies may become more noticeable.

Additionally, prior to ZFSonLinux 0.7, the `zpool list` command did not have a
display option to show values in bytes (`-p`), so on distributions like Ubuntu
16.04, the plugin must convert human-readable figures, e.g., "100G" to a byte
value. This loses precision, but is unavoidable.
