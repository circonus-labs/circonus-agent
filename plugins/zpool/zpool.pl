#!/usr/bin/env perl
#
# ATTENTION LINUX USERS:
# Because circonus-agent normally runs as an unprivileged user, it may not be
# able to run "zpool {status,list}" without root privileges, due to the
# permissions on /dev/zfs.
# This was addressed in ZFSonLinux 0.7.0 and later.
# See https://github.com/zfsonlinux/zfs/issues/362 for a potential workaround
# if your distribution does not support unprivileged access to /dev/zfs.
#
# Metric descriptions
# See also the zpool man page
#
# zpool status:
#
# errors|ST[type:read,pool:<poolname>,collector:zpool,source:circonus-agent]
# errors|ST[type:write,pool:<poolname>,collector:zpool,source:circonus-agent]
# errors|ST[type:cksum,pool:<poolname>,collector:zpool,source:circonus-agent]
#    read/write/checksum errors at the pool level
#
# errors|ST[vdev:<device>,type:read,pool:<poolname>,collector:zpool,source:circonus-agent]
# errors|ST[vdev:<device>,type:write,pool:<poolname>,collector:zpool,source:circonus-agent]
# errors|ST[vdev:<device>,type:cksum,pool:<poolname>,collector:zpool,source:circonus-agent]
#    read/write/checksum errors at the vdev level
#
# scan_done|ST[type:resilver,units:percent,pool:<poolname>,collector:zpool,source:circonus-agent]
#    percent done for an ongoing resilver action
#    0 means no resilver active, or started so recently that it has not made significant progress yet
#
# scan_done|ST[type:scrub,units:percent,pool:<poolname>,collector:zpool,source:circonus-agent]
#    percent done for an ongoing scrub action
#    0 means no scrub active, or started so recently that it has not made significant progress yet
#
# scan_remaining|ST[type:resilver,units:seconds,pool:<poolname>,collector:zpool,source:circonus-agent]
#    time remaining for an ongoing resilver action, in seconds
#    0 means no resilver active, -1 means progress is too slow (ETR > 30 days or issue rate < 10 MB/s)
#
# scan_remaining|ST[type:scrub,units:seconds,pool:<poolname>,collector:zpool,source:circonus-agent]
#    time remaining for an ongoing scrub action, in seconds
#    0 means no scrub active, -1 means progress is too slow (ETR > 30 days or issue rate < 10 MB/s)
#
# state|ST[pool:<poolname>,collector:zpool,source:circonus-agent]
#    overall pool state, e.g., ONLINE
#
# state|ST[vdev:<device>,pool:<poolname>,collector:zpool,source:circonus-agent]
#    individual vdev state
#
# zpool list:
#
# alloc|ST[units:bytes,pool:<poolname>,collector:zpool,source:circonus-agent]
#    bytes allocated in the pool
#    NOTE: not the same as USED from `zfs list`
#
# capacity|ST[units:percent,pool:<poolname>,collector:zpool,source:circonus-agent]
#    percentage of pool capacity that is in use (alloc/size)
#
# dedup|ST[units:ratio,pool:<poolname>,collector:zpool,source:circonus-agent]
#    deduplication ratio (1.00 when dedup is not in use)
#
# free|ST[units:bytes,pool:<poolname>,collector:zpool,source:circonus-agent]
#    bytes free in the pool
#    NOTE: not the same as AVAIL-USED from `zfs list`
#
# frag|ST[pool:<poolname>,collector:zpool,source:circonus-agent]
#    average of metaslab free space fragmentation
#    higher values indicate the available space is made up of relatively small segments
#
# size|ST[units:bytes,pool:<poolname>,collector:zpool,source:circonus-agent]
#    pool size, in bytes
#    NOTE: includes space used for parity/redundancy, not the same as USED+AVAIL from `zfs list`

use warnings;
use strict;
use POSIX qw/floor/;

my $common_tags = 'collector:zpool,source:circonus-agent';
my @pools = `/sbin/zpool list -H -o name`;

# Accumulate metrics for printing at the end of each pool iteration
# $metrics->{pool}{tagged_metric}{type}
# $metrics->{pool}{tagged_metric}{value}
my $metrics = {};

# Older ZFS (ZoL 0.6.x, for one) did not have a parseable option (-p)
# so we'll need to convert the pretty-printed figures, at the cost of
# some precision.
my $zpool_list_has_p;
my @test_zpool_list_p = qq(/sbin/zpool list -p >/dev/null 2>&1);
if (system(@test_zpool_list_p) == 0) { $zpool_list_has_p = 1; }

foreach my $pool (@pools) {
    chomp $pool;

    my $pool_tag        = "pool:$pool";
    my $resilver_remain = make_stream('scan_remaining', 'type:resilver', 'units:seconds', $pool_tag);
    my $resilver_done   = make_stream('scan_done', 'type:resilver', 'units:percent', $pool_tag);
    my $scrub_remain    = make_stream('scan_remaining', 'type:scrub', 'units:seconds', $pool_tag);
    my $scrub_done      = make_stream('scan_done', 'type:scrub', 'units:percent', $pool_tag);

    $metrics->{$pool}{$resilver_remain}{'type'} = 'l';
    $metrics->{$pool}{$resilver_done}{'type'}   = 'n';
    $metrics->{$pool}{$scrub_remain}{'type'}    = 'l';
    $metrics->{$pool}{$scrub_done}{'type'}      = 'n';

    my ($in_scrub, $in_resilver, $scan_remain, $scan_done);

    open ZPOOL_STATUS, "/sbin/zpool status $pool |";
    while (<ZPOOL_STATUS>) {
        chomp $_;
        my $line = $_;

        next if $line =~ /^\s+(?:pool:|config:|errors:|status:|action:|NAME|logs$|cache$)/;

        if($line =~ /^\s+state: (\S+)$/) {
            my $pool_state = $1;
            my $st = make_stream('state', $pool_tag);

            $metrics->{$pool}{$st}{'type'} = 's';
            $metrics->{$pool}{$st}{'value'} = $pool_state;

            next;
        }

        if ($line =~ /^\s+scan: (\S+) (\S+ \S+)/) {
            if ($2 eq 'in progress') {
                if    ($1 eq 'scrub')    { $in_scrub = 1; }
                elsif ($1 eq 'resilver') { $in_resilver = 1; }
            }

            # default to 0 for scan stats, initiallly
            $metrics->{$pool}{$resilver_remain}{'value'} = 0;
            $metrics->{$pool}{$resilver_done}{'value'}   = 0;
            $metrics->{$pool}{$scrub_remain}{'value'}    = 0;
            $metrics->{$pool}{$scrub_done}{'value'}      = 0;
            
            next;
        }

        # The sequential scrub/resilver feature changed the format,
        # so the estimate could appear on one of two lines, with or
        # without the %done.
        if ($line =~ /(scanned|to go)/) {
            if ($line =~ /([\d\.]+)% done.*(\d+) days (\d{2}):(\d{2}):(\d{2}) to go$/) {  # new format
                $scan_done = $1;
                my $days   = $2;
                my $hours  = $3;
                my $mins   = $4;
                my $secs   = $5;

                $scan_remain = ($days * 86400) + ($hours * 3600) + ($mins * 60) + $secs;
            } elsif ($line =~ /(\d+)h(\d+)m to go$/) {  # old format
                my $hours = $1;
                my $mins  = $2;

                $scan_remain = ($hours * 3600) + ($mins * 60);
            } elsif ($line =~ /no estimated/) {  # old format, scan too slow
                $scan_remain = -1;
            }

            # Assign values that we've got so far. May change depending on
            # the next line, for certain combinations of format + situation.
            if ($in_scrub) {
                $metrics->{$pool}{$scrub_remain}{'value'} = $scan_remain;
                $metrics->{$pool}{$scrub_done}{'value'}   = $scan_done;
            } elsif ($in_resilver) {
                $metrics->{$pool}{$resilver_remain}{'value'} = $scan_remain;
                $metrics->{$pool}{$resilver_done}{'value'}   = $scan_done;
            }
        } elsif (! defined $scan_done && $line =~ /([\d\.]+)% done/) {
            # %done is on its own line in the old format
            $scan_done = $1;

            if ($line =~ /no estimated/) {  # new format, scan too slow
                $scan_remain = -1;
            }

            if ($in_scrub) {
                $metrics->{$pool}{$scrub_remain}{'value'} = $scan_remain;
                $metrics->{$pool}{$scrub_done}{'value'}   = $scan_done;
            } elsif ($in_resilver) {
                $metrics->{$pool}{$resilver_remain}{'value'} = $scan_remain;
                $metrics->{$pool}{$resilver_done}{'value'}   = $scan_done;
            }
        }

        # stats lines that we always have
        if ($line =~ /^\s+(\S+)\s+([A-Z]+)\s+(\d+[KMG]?)\s+(\d+[KMG]?)\s+(\d+[KMG]?)/) {
            my $dev        = $1;
            my $dev_state  = $2;
            my $read_errs  = $3;
            my $write_errs = $4;
            my $cksum_errs = $5;

            $read_errs = convert_base($read_errs, 10);
            $write_errs = convert_base($write_errs, 10);
            $cksum_errs = convert_base($cksum_errs, 10);

            if ($dev eq $pool) {
                # pool-wide error stats (state already recorded above)
                my $pool_rerr = make_stream('errors', 'type:read', $pool_tag);
                my $pool_werr = make_stream('errors', 'type:write', $pool_tag);
                my $pool_cerr = make_stream('errors', 'type:cksum', $pool_tag);

                $metrics->{$pool}{$pool_rerr}{'type'} = 'L';
                $metrics->{$pool}{$pool_rerr}{'value'} = $read_errs;
                $metrics->{$pool}{$pool_werr}{'type'} = 'L';
                $metrics->{$pool}{$pool_werr}{'value'} = $write_errs;
                $metrics->{$pool}{$pool_cerr}{'type'} = 'L';
                $metrics->{$pool}{$pool_cerr}{'value'} = $cksum_errs;
            } else {
                # per-vdev stats
                my $vdev_state = make_stream('state', "vdev:$dev", $pool_tag);
                my $vdev_rerr  = make_stream('errors', "vdev:$dev", 'type:read', $pool_tag);
                my $vdev_werr  = make_stream('errors', "vdev:$dev", 'type:write', $pool_tag);
                my $vdev_cerr  = make_stream('errors', "vdev:$dev", 'type:cksum', $pool_tag);

                $metrics->{$pool}{$vdev_state}{'type'} = 's';
                $metrics->{$pool}{$vdev_state}{'value'} = $dev_state;
                $metrics->{$pool}{$vdev_rerr}{'type'} = 'L';
                $metrics->{$pool}{$vdev_rerr}{'value'} = $read_errs;
                $metrics->{$pool}{$vdev_werr}{'type'} = 'L';
                $metrics->{$pool}{$vdev_werr}{'value'} = $write_errs;
                $metrics->{$pool}{$vdev_cerr}{'type'} = 'L';
                $metrics->{$pool}{$vdev_cerr}{'value'} = $cksum_errs;
            }
        }
    }

    close ZPOOL_STATUS;

    # If we didn't find any scrub/resilver in progress, set values to 0
    if (! defined $scan_done && ! defined $scan_remain) {
        $metrics->{$pool}{$scrub_remain}{'value'}    = 0;
        $metrics->{$pool}{$scrub_done}{'value'}      = 0;
        $metrics->{$pool}{$resilver_remain}{'value'} = 0;
        $metrics->{$pool}{$resilver_done}{'value'}   = 0;
    }

    # stats from zpool list (capacity, frag, etc.)

    my $size  = make_stream('size', 'units:bytes', $pool_tag);
    my $alloc = make_stream('alloc', 'units:bytes', $pool_tag);
    my $free  = make_stream('free', 'units:bytes', $pool_tag);
    my $frag  = make_stream('frag', $pool_tag);
    my $cap   = make_stream('capacity', 'units:percent', $pool_tag);
    my $dedup = make_stream('dedup', 'units:ratio', $pool_tag);

    $metrics->{$pool}{$size}{'type'} = 'L';
    $metrics->{$pool}{$alloc}{'type'} = 'L';
    $metrics->{$pool}{$free}{'type'} = 'L';
    $metrics->{$pool}{$frag}{'type'} = 'I';
    $metrics->{$pool}{$cap}{'type'} = 'I';
    $metrics->{$pool}{$dedup}{'type'} = 'n';

    my $zp_list;
    if ($zpool_list_has_p) {
        $zp_list = '/sbin/zpool list -H -p';
    } else {
        $zp_list = '/sbin/zpool list -H';
    }

    open ZPOOL_LIST, "$zp_list $pool |";
    while (<ZPOOL_LIST>) {
        chomp $_;

        my @line = split(/\s+/, $_);

        # column indexes
        my $col_size  = 1;
        my $col_alloc = 2;
        my $col_free  = 3;

        # pools with the "zpool_checkpoint" feature have an extra column after FREE
        my ($col_frag, $col_cap, $col_dedup);
        if (scalar @line == 11) {
            $col_frag  = 6;
            $col_cap   = 7;
            $col_dedup = 8;
        } else {
            $col_frag  = 5;
            $col_cap   = 6;
            $col_dedup = 7;
        }

        if ($zpool_list_has_p) {
            $metrics->{$pool}{$size}{'value'}  = $line[$col_size];
            $metrics->{$pool}{$alloc}{'value'} = $line[$col_alloc];
            $metrics->{$pool}{$free}{'value'}  = $line[$col_free];
        } else {
            $metrics->{$pool}{$size}{'value'}  = convert_base($line[$col_size], 2);
            $metrics->{$pool}{$alloc}{'value'} = convert_base($line[$col_alloc], 2);
            $metrics->{$pool}{$free}{'value'}  = convert_base($line[$col_free], 2);
        }

        (my $frag_val = $line[$col_frag]) =~ s/%//;
        $metrics->{$pool}{$frag}{'value'}  = $frag_val;

        (my $cap_val = $line[$col_cap]) =~ s/%//;
        $metrics->{$pool}{$cap}{'value'}  = $cap_val;

        (my $dedup_val = $line[$col_dedup]) =~ s/x//;
        $metrics->{$pool}{$dedup}{'value'}  = $dedup_val;
    }

    close ZPOOL_LIST;

    # Done gathering metrics.
    # Emit all the stuff for this pool, sorted by metric name for consistency
    map {
        printf("%s\t%s\t%s\n",
            $_,
            $metrics->{$pool}{$_}{'type'}, 
            $metrics->{$pool}{$_}{'value'}
        );
    } sort keys %{$metrics->{$pool}}; 
}

sub convert_base {
    my ($passed, $base) = @_;
    my ($value, $unit, $converted);

    my %base2 = (
        'K' => 1024,
        'M' => 1048576,
        'G' => 1073741824,
        'T' => 1099511627776,
        'P' => 1125899906842624,
    );
    my %base10 = (
        'K' => 1000,
        'M' => 1000000,
        'G' => 1000000000,
        'T' => 1000000000000,
        'P' => 1000000000000000,
    );

    if ($passed =~ /^([\d+\.]+)([KMGTP])$/) {
        $value = $1;
        $unit  = $2;

        if ($base == 2)  {
            $converted = floor($value * $base2{$unit});
        } elsif ($base == 10) {
            $converted = floor($value * $base10{$unit});
        }

        # Translate any really big numbers from scientific notation
        if ($converted =~ /e\+\d+/) {
            $converted = sprintf("%.10d", $converted);
        }
    } elsif ($passed =~ /^\d+$/) {
        # Bare value
        $converted = $passed;
    } else {
        # For some reason we didn't match, but we should not return undef
        $converted = '0';
    }

    return $converted;
}

# Construct a stream-tagged metric name.
# Only need to pass in the specific tags for this stream;
# this will always append the collector/source tags.
sub make_stream {
    my $base = shift;
    my @tags = @_;

    my $taglist = join(',', @tags, $common_tags);
    my $stream = $base.'|ST['.$taglist.']';

    return $stream;
}
