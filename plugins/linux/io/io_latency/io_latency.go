package main

import "fmt"
import "os"
import "log"
import "strconv"
import "strings"
import "syscall"
import "time"
import "encoding/json"

import "github.com/circonus-labs/circonusllhist"

import "../event_harness"

var MAX_AGE = 310.0 // 5m10s
var devlist = map[string]string{}

var inserts = map[string]float64{}
var issues = map[string]float64{}
var max_whence = float64(0)
var latencies = map[string]interface{}{}
var cleantables = time.Ticker{}

func trackLatency(dev, typ string, duration float64) {
	var latency map[string]*circonusllhist.Histogram
	if latency_gen, ok := latencies[typ]; ok {
		latency = latency_gen.(map[string]*circonusllhist.Histogram)
	} else {
		latency = make(map[string]*circonusllhist.Histogram)
		latencies[typ] = latency
	}
	hist, ok2 := latency[dev]
	if !ok2 {
		latency[dev] = circonusllhist.NewNoLocks()
		hist = latency[dev]
	}
	hist.RecordValue(duration)
}

func dumpHistAndClear() {
	tmp := map[string]interface{}{}
	for typ, v := range latencies {
		latency := v.(map[string]*circonusllhist.Histogram)
		for dev, hist := range latency {
			if ss := hist.DecStrings(); len(ss) > 0 {
				hist.Reset()
	  			val := make(map[string]interface{})
	  			val["_type"] = "n"
	  			val["_value"] = ss
				tmp[typ + "|ST[device:" + dev + "]"] = val
			}
		}
	}
	if buff, err := json.Marshal(tmp); err == nil {
		fmt.Printf("%s\n\n", string(buff))
	}
}

func handleLogLine(line string) {
	parts := strings.Fields(line)
	if len(parts) >= 11 && devlist[parts[5]] != "" {
		whence, err := strconv.ParseFloat(strings.Replace(parts[3], ":", "", 1), 64)
		if err != nil {
			return
		}
		if whence > max_whence {
			max_whence = whence
		}
		op, dev := strings.Replace(parts[4], ":", "", 1), devlist[parts[5]]
		if whence == 0 {
			return
		}
		select {
		case <-cleantables.C:
			for k, v := range inserts {
				if max_whence-v > MAX_AGE {
					delete(inserts, k)
				}
			}
			for k, v := range issues {
				if max_whence-v > MAX_AGE {
					delete(inserts, k)
				}
			}
		default:
		}
		switch {
		case op == "block_rq_insert":
			sec, nsec := parts[9], parts[11]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			inserts[opkey] = whence
		case op == "block_rq_issue":
			sec, nsec := parts[9], parts[11]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			if start, ok := inserts[opkey]; ok {
				issues[opkey] = whence
				trackLatency(dev, "q2d", whence-start)
			}
		case op == "block_rq_complete":
			sec, nsec := parts[8], parts[10]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			if start, ok := inserts[opkey]; ok {
				trackLatency(dev, "q2c", whence-start)
				delete(inserts, opkey)
			}
			if start, ok := issues[opkey]; ok {
				trackLatency(dev, "c2d", whence-start)
				delete(issues, opkey)
			}
		default:
		}
	}
}

func refreshDevs() {
	dir, err := os.Open("/dev/")
	if err != nil {
		return
	}
	files, err := dir.Readdir(0)
	if err != nil {
		return
	}
	for _, file := range files {
		if stat := file.Sys().(*syscall.Stat_t); file.Mode()&os.ModeDevice != 0 && file.Mode()&os.ModeCharDevice == 0 && stat != nil {
			major, minor := stat.Rdev/256, stat.Rdev%256
			key := fmt.Sprintf("%d,%d", major, minor)
			if _, ok := devlist[key]; !ok {
				devlist[key] = file.Name()
			}
		}
	}
}
func cleanupTables() {
	for k, v := range inserts {
		if max_whence-v > MAX_AGE {
			delete(inserts, k)
		}
	}
	for k, v := range issues {
		if max_whence-v > MAX_AGE {
			delete(inserts, k)
		}
	}
}
func main() {
	refreshDevs()

	h, err := event_harness.HarnessMain("circ_blk",
		[][]string{[]string{"events/block/block_rq_issue/enable", "1\n"},
			[]string{"events/block/block_rq_insert/enable", "1\n"},
			[]string{"events/block/block_rq_complete/enable", "1\n"}},
		handleLogLine)
	if err != nil {
		log.Fatalf("Cannot start tracing.\n%s\n", err)
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			h.Tasks <- cleanupTables
			h.Tasks <- dumpHistAndClear
		}
	}()

	err = <-h.Done
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}
}
