package whanau

import "testing"
import "runtime"
import "strconv"
import "os"
import "fmt"
import "math/rand"
import "math"
import "time"
import crand "crypto/rand"
import "crypto/rsa"

func port(tag string, host int) string {
	s := "/var/tmp/824-"
	s += strconv.Itoa(os.Getuid()) + "/"
	os.Mkdir(s, 0777)
	s += "sm-"
	s += strconv.Itoa(os.Getpid()) + "-"
	s += tag + "-"
	s += strconv.Itoa(host)
	return s
}

func cleanup(ws []*WhanauServer) {
	for i := 0; i < len(ws); i++ {
		if ws[i] != nil {
			ws[i].Kill()
		}
	}
}

// TODO just for testing
func testRandomWalk(server string, steps int) string {
	args := &RandomWalkArgs{}
	args.Steps = steps
	var reply RandomWalkReply
	ok := call(server, "WhanauServer.RandomWalk", args, &reply)
	if ok && (reply.Err == OK) {
		return reply.Server
	}

	return "RANDOMWALK ERR"
}

// Test getID
func testGetId(server string, layer int) KeyType {
	args := &GetIdArgs{}
	args.Layer = layer
	var reply GetIdReply
	ok := call(server, "WhanauServer.GetId", args, &reply)
	if ok && (reply.Err == OK) {
		return reply.Key
	}

	return "GETID ERR"
}

/*
func TestBasic(t *testing.T) {
	runtime.GOMAXPROCS(4)

	const nservers = 3
	var ws []*WhanauServer = make([]*WhanauServer, nservers)
	var kvh []string = make([]string, nservers)
	defer cleanup(ws)

	for i := 0; i < nservers; i++ {
		kvh[i] = port("basic", i)
	}

	for i := 0; i < nservers; i++ {
		neighbors := make([]string, 0)
		for j := 0; j < nservers; j++ {
			if j == i {
				continue
			}
			neighbors = append(neighbors, kvh[j])
		}

		ws[i] = StartServer(kvh, i, kvh[i], neighbors)
	}

	var cka [nservers]*Clerk
	for i := 0; i < nservers; i++ {
		cka[i] = MakeClerk(kvh[i])
	}

	fmt.Printf("Test: Basic put/lookup ...\n")

	cka[1].Put("a", "x")
	val := cka[1].Lookup("a")

	fmt.Printf("lookup for key a got value %s\n", val)

	fmt.Printf("...Passed\n")

	fmt.Printf("Lookup in neighboring server ...\n")

	cka[2].Put("b", "y")
	val = cka[1].Lookup("b")

	fmt.Printf("lookup for key b got value %s\n", val)

	fmt.Printf("...Passed\n")
}
*/

func TestLookup(t *testing.T) {
	runtime.GOMAXPROCS(8)

	const nservers = 10
	const nkeys = 50           // keys are strings from 0 to 99
	const k = nkeys / nservers // keys per node

	// run setup in parallel
	// parameters
	constant := 5
	nlayers := int(math.Log(float64(k*nservers))) + 1
	nfingers := int(math.Sqrt(k * nservers))
	w := constant * int(math.Log(float64(nservers))) // number of steps in random walks, O(log n) where n = nservers
	rd := 2 * int(math.Sqrt(k*nservers))             // number of records in the db
	rs := constant * int(math.Sqrt(k*nservers))      // number of nodes to sample to get successors
	ts := 5                                          // number of successors sampled per node

	var ws []*WhanauServer = make([]*WhanauServer, nservers)
	var kvh []string = make([]string, nservers)
	defer cleanup(ws)

	for i := 0; i < nservers; i++ {
		kvh[i] = port("basic", i)
	}

	for i := 0; i < nservers; i++ {
		neighbors := make([]string, 0)
		for j := 0; j < nservers; j++ {
			if j == i {
				continue
			}
			neighbors = append(neighbors, kvh[j])
		}

		ws[i] = StartServer(kvh, i, kvh[i], neighbors, make([]string, 0), false, false,
			nlayers, nfingers, w, rd, rs, ts)
	}

	var cka [nservers]*Clerk
	for i := 0; i < nservers; i++ {
		cka[i] = MakeClerk(kvh[i])
	}

	fmt.Printf("\033[95m%s\033[0m\n", "Test: Lookup")

	keys := make([]KeyType, 0)
	records := make(map[KeyType]ValueType)
	counter := 0
	// hard code in records for each server
	for i := 0; i < nservers; i++ {
		for j := 0; j < nkeys/nservers; j++ {
			//var key KeyType = testKeys[counter]
			var key KeyType = KeyType(strconv.Itoa(counter))
			keys = append(keys, key)
			counter++
			val := ValueType{}
			// randomly pick 5 servers
			for kp := 0; kp < PaxosSize; kp++ {
				val.Servers = append(val.Servers, "ws"+strconv.Itoa(rand.Intn(PaxosSize)))
			}
			records[key] = val
			ws[i].kvstore[key] = val
		}
	}
	/*
		for i := 0; i < nservers; i++ {
			fmt.Printf("ws[%d].kvstore: %s\n", i, ws[i].kvstore)
		}
	*/
	c := make(chan bool) // writes true of done
	fmt.Printf("Starting setup\n")
	start := time.Now()
	for i := 0; i < nservers; i++ {
		go func(srv int) {
			ws[srv].Setup()
			c <- true
		}(i)
	}

	// wait for all setups to finish
	for i := 0; i < nservers; i++ {
		done := <-c
		DPrintf("ws[%d] setup done: %b", i, done)
	}

	elapsed := time.Since(start)
	fmt.Printf("Finished setup, time: %s\n", elapsed)

	/*
		for i := 0; i < nservers; i++ {
			fmt.Println("")
			//fmt.Printf("ws[%d].db: %s\n", i, ws[i].db)

			fmt.Printf("ws[%d].ids[%d]: %s\n", i, 0, ws[i].ids[0])
			for j := 1; j < nlayers; j++ {
				fmt.Printf("ws[%d].fingers[%d]: %s\n", i, j-1, ws[i].fingers[j-1])
				fmt.Printf("ws[%d].ids[%d]: %s\n\n", i, j, ws[i].ids[j])
				//fmt.Printf("ws[%d].succ[%d]: %s\n", i, j, ws[i].succ[j])
			}
		}
	*/

	fmt.Printf("Check key coverage in all dbs\n")

	keyset := make(map[KeyType]bool)
	for i := 0; i < len(keys); i++ {
		keyset[keys[i]] = false
	}

	for i := 0; i < nservers; i++ {
		srv := ws[i]
		for j := 0; j < len(srv.db); j++ {
			keyset[srv.db[j].Key] = true
		}
	}

	// count number of covered keys, all the false keys in keyset
	covered_count := 0
	for _, v := range keyset {
		if v {
			covered_count++
		}
	}
	fmt.Printf("key coverage in all dbs: %f\n", float64(covered_count)/float64(len(keys)))

	fmt.Printf("Check key coverage in all successor tables\n")
	keyset = make(map[KeyType]bool)
	for i := 0; i < len(keys); i++ {
		keyset[keys[i]] = false
	}

	for i := 0; i < nservers; i++ {
		srv := ws[i]
		for j := 0; j < len(srv.succ); j++ {
			for k := 0; k < len(srv.succ[j]); k++ {
				keyset[srv.succ[j][k].Key] = true
			}
		}
	}

	// count number of covered keys, all the false keys in keyset
	covered_count = 0
	missing_keys := make([]KeyType, 0)
	for k, v := range keyset {
		if v {
			covered_count++
		} else {
			missing_keys = append(missing_keys, k)
		}
	}

	fmt.Printf("key coverage in all succ: %f\n", float64(covered_count)/float64(len(keys)))
	fmt.Printf("missing keys in succs: %s\n", missing_keys)
	// check populated ids and fingers
	/*
		var x0 KeyType = "1"
		var key KeyType = "3"
		finger, layer := ws[0].ChooseFinger(x0, key, nlayers)
		fmt.Printf("chosen finger: %s, chosen layer: %d\n", finger, layer)
	*/

	fmt.Printf("Checking Try for every key from every node\n")
	numFound := 0
	numTotal := 0
	ctr := 0
	fmt.Printf("All test keys: %s\n", keys)
	for i := 0; i < nservers; i++ {
		for j := 0; j < len(keys); j++ {
			key := KeyType(keys[j])
			ctr++
			largs := &LookupArgs{key, nil}
			lreply := &LookupReply{}
			ws[i].Lookup(largs, lreply)
			if lreply.Err != OK {
				//fmt.Printf("Did not find key: %s\n", key)
			} else {
				value := lreply.Value
				// compare string arrays...
				if len(value.Servers) != len(records[key].Servers) {
					t.Fatalf("Wrong value returned (length test): %s expected: %s", value, records[key])
				}
				for k := 0; k < len(value.Servers); k++ {
					if value.Servers[k] != records[key].Servers[k] {
						t.Fatalf("Wrong value returned for key(%s): %s expected: %s", key, value, records[key])
					}
				}
				numFound++
			}
			numTotal++
		}
	}

	fmt.Printf("numFound: %d\n", numFound)
	fmt.Printf("total keys: %d\n", nkeys)
	fmt.Printf("Percent lookups successful: %f\n", float64(numFound)/float64(numTotal))

}

// Test a basic put/get using paxos without checking lookup integrity
func TestPutGet(t *testing.T) {
	runtime.GOMAXPROCS(4)

	fmt.Printf("\033[95m%s\033[0m\n", "Test: Basic PutGet")
	const nservers = 10
	const nkeys = 20           // keys are strings from 0 to 99
	const k = nkeys / nservers // keys per node

	// run setup in parallel
	// parameters
	constant := 5
	nlayers := constant*int(math.Log(float64(k*nservers))) + 1
	nfingers := constant * int(math.Sqrt(k*nservers))
	w := constant * int(math.Log(float64(nservers))) // number of steps in random walks, O(log n) where n = nservers
	rd := constant * int(math.Sqrt(k*nservers))      // number of records in the db
	rs := constant * int(math.Sqrt(k*nservers))      // number of nodes to sample to get successors
	ts := constant                                   // number of successors sampled per node

	var ws []*WhanauServer = make([]*WhanauServer, nservers)
	var kvh []string = make([]string, nservers)
	defer cleanup(ws)

	for i := 0; i < nservers; i++ {
		kvh[i] = port("basic", i)
	}

	for i := 0; i < nservers; i++ {
		neighbors := make([]string, 0)
		for j := 0; j < nservers; j++ {
			if j == i {
				continue
			}
			neighbors = append(neighbors, kvh[j])
		}

		ws[i] = StartServer(kvh, i, kvh[i], neighbors, make([]string, 0), false, false,
			nlayers, nfingers, w, rd, rs, ts)
	}

	var cka [nservers]*Clerk
	for i := 0; i < nservers; i++ {
		cka[i] = MakeClerk(kvh[i])
	}

}

func TestDataIntegrityBasic(t *testing.T) {
	runtime.GOMAXPROCS(4)

	fmt.Printf("\033[95m%s\033[0m\n", "Test: Data Integrity Functions")
	sk, err := rsa.GenerateKey(crand.Reader, 2014)

	if err != nil {
		t.Fatalf("key gen err", err)
	}

	err = sk.Validate()
	if err != nil {
		t.Fatalf("Validation failed.", err)
	}

	fmt.Println("Testing verification on true value type")
	val1 := TrueValueType{"testval", "srv1", nil, &sk.PublicKey}

	sig2, _ := SignTrueValue(val1, sk)
	val1.Sign = sig2

	if VerifyTrueValue(val1) {
		fmt.Println("true value verified!")
	} else {
		t.Fatalf("TrueValue couldn't verify")
	}

	val1.TrueValue = "changed"
	if !VerifyTrueValue(val1) {
		fmt.Println("true value modification detected!")
	} else {
		t.Fatalf("True value modification not detected")
	}

	sk1, _ := rsa.GenerateKey(crand.Reader, 2014)

	val1 = TrueValueType{"testval", "srv1", nil, &sk1.PublicKey}
	val1.Sign = sig2

	if !VerifyTrueValue(val1) {
		fmt.Println("true value pk modification detected!")
	} else {
		t.Fatalf("True value PK modification not detected")
	}

}

func TestRealGetAndPut(t *testing.T) {

	runtime.GOMAXPROCS(4)

	const nservers = 10
	const nkeys = 20           // keys are strings from 0 to 99
	const k = nkeys / nservers // keys per node

	// parameters
	constant := 5
	nlayers := constant*int(math.Log(float64(k*nservers))) + 1
	nfingers := constant * int(math.Sqrt(k*nservers))
	w := constant * int(math.Log(float64(nservers))) // number of steps in random walks, O(log n) where n = nservers
	rd := constant * int(math.Sqrt(k*nservers))      // number of records in the db
	rs := constant * int(math.Sqrt(k*nservers))      // number of nodes to sample to get successors
	ts := constant                                   // number of successors sampled per node

	fmt.Printf("nlayers is %u, w is %u\n", nlayers, w)

	var ws []*WhanauServer = make([]*WhanauServer, nservers)
	var kvh []string = make([]string, nservers)
	defer cleanup(ws)

	for i := 0; i < nservers; i++ {
		kvh[i] = port("basic", i)
	}

	master_servers := []string{kvh[0], kvh[1], kvh[2]}

	for i := 0; i < nservers; i++ {
		neighbors := make([]string, 0)
		for j := 0; j < nservers; j++ {
			if j == i {
				continue
			}
			neighbors = append(neighbors, kvh[j])
		}

		if i < 3 {
			ws[i] = StartServer(kvh, i, kvh[i], neighbors, master_servers, true, false,
				nlayers, nfingers, w, rd, rs, ts)
		} else {
			ws[i] = StartServer(kvh, i, kvh[i], neighbors, master_servers, false, false,
				nlayers, nfingers, w, rd, rs, ts)
		}
	}

	var cka [nservers]*Clerk
	for i := 0; i < nservers; i++ {
		cka[i] = MakeClerk(kvh[i])
	}

	fmt.Printf("\033[95m%s\033[0m\n", "Test: Real Lookup")

	keys := make([]KeyType, 0)
	records := make(map[KeyType]ValueType)
	counter := 0
	// hard code in records for each server
	for i := 0; i < nservers; i++ {

		paxos_cluster := []string{kvh[i], kvh[(i+1)%nservers], kvh[(i+2)%nservers]}
		wp0 := StartWhanauPaxos(paxos_cluster, 0, ws[i].rpc)
		wp1 := StartWhanauPaxos(paxos_cluster, 1, ws[(i+1)%nservers].rpc)
		wp2 := StartWhanauPaxos(paxos_cluster, 2, ws[(i+2)%nservers].rpc)

		for j := 0; j < nkeys/nservers; j++ {
			//var key KeyType = testKeys[counter]
			var key KeyType = KeyType(strconv.Itoa(counter))
			keys = append(keys, key)
			counter++

			fmt.Printf("paxos_cluster is %v\n", paxos_cluster)
			val := ValueType{paxos_cluster}
			records[key] = val
			ws[i].kvstore[key] = val

			ws[i].paxosInstances[key] = *wp0
			ws[(i+1)%nservers].paxosInstances[key] = *wp1
			ws[(i+2)%nservers].paxosInstances[key] = *wp2

			val0 := TrueValueType{"hello", wp0.myaddr, nil, &ws[i].secretKey.PublicKey}
			sig0, _ := SignTrueValue(val0, ws[i].secretKey)
			val0.Sign = sig0
			wp0.db[key] = val0

			val1 := TrueValueType{"hello", wp1.myaddr, nil, &ws[(i+1)%nservers].secretKey.PublicKey}
			sig1, _ := SignTrueValue(val1, ws[(i+1)%nservers].secretKey)
			val1.Sign = sig1
			wp1.db[key] = val1

			val2 := TrueValueType{"hello", wp2.myaddr, nil, &ws[(i+2)%nservers].secretKey.PublicKey}
			sig2, _ := SignTrueValue(val2, ws[(i+2)%nservers].secretKey)
			val2.Sign = sig2
			wp2.db[key] = val2
		}
	}

	c := make(chan bool) // writes true of done
	fmt.Printf("Starting setup\n")
	start := time.Now()
	for i := 0; i < nservers; i++ {
		go func(srv int) {
			DPrintf("running ws[%d].Setup", srv)
			ws[srv].Setup()
			c <- true
		}(i)
	}

	// wait for all setups to finish
	for i := 0; i < nservers; i++ {
		done := <-c
		DPrintf("ws[%d] setup done: %b", i, done)
	}

	elapsed := time.Since(start)
	fmt.Printf("Finished setup, time: %s\n", elapsed)

	// start clients

	largs := &LookupArgs{"0", nil}
	lreply := &LookupReply{}
	ws[3].Lookup(largs, lreply)
	fmt.Printf("lreply.value is %v\n", lreply.Value.Servers)

	cl := MakeClerk(kvh[0])

	fmt.Printf("Try to do a lookup from client\n")

	value := cl.ClientGet("0")
	fmt.Printf("value is %s\n", value)

	// test single value put -- an update, NOT an insert!

	cl.ClientPut("0", "helloworld")
	value = cl.ClientGet("0")

	fmt.Printf("After put: value is %v\n", value)
}

// Test doesn't work, wrote it for subclassing sybil nodes
/*
func TestRealGetAndPutWithSybils(t *testing.T) {
	runtime.GOMAXPROCS(4)

	const nservers = 10
	const nkeys = 20           // keys are strings from 0 to 99
	const k = nkeys / nservers // keys per node
    const nsybilservers = 10
    const sybilConnectionProb = 0.7

	// parameters
	constant := 5
	nlayers := constant*int(math.Log(float64(k*nservers))) + 1
	nfingers := constant * int(math.Sqrt(k*nservers))
	w := constant * int(math.Log(float64(nservers))) // number of steps in random walks, O(log n) where n = nservers
	rd := constant * int(math.Sqrt(k*nservers))      // number of records in the db
	rs := constant * int(math.Sqrt(k*nservers))      // number of nodes to sample to get successors
	ts := constant                                   // number of successors sampled per node

	fmt.Printf("nlayers is %u, w is %u\n", nlayers, w)

	var ws []*WhanauServer = make([]*WhanauServer, nservers)
	var kvh []string = make([]string, nservers)
    var ksvh []string = make([]string, nsybilservers)
    var wss []*WhanauSybilServer = make([]*WhanauSybilServer, nsybilservers)
    var sybilnormalneighbors [nsybilservers][]string
	defer cleanup(ws)

	for i := 0; i < nservers; i++ {
		kvh[i] = port("basic", i)
	}

    for i := nservers; i < nsybilservers + nservers; i++ {
        ksvh[i-nservers] = port("basic", i)
    }

	master_servers := []string{kvh[0], kvh[1], kvh[2]}

	for i := 0; i < nservers; i++ {
		neighbors := make([]string, 0)
		for j := 0; j < nservers; j++ {
			if j == i {
				continue
			}
			neighbors = append(neighbors, kvh[j])
		}

        rand.Seed(time.Now().UTC().UnixNano())

        for k := 0; k < nsybilservers; k++ {
            prob := rand.Float32()
*/
