// Routing functions for Whanau

package whanau

import "math/rand"

// Random walk
func (ws *WhanauServer) RandomWalk(args *RandomWalkArgs, reply *RandomWalkReply) error {
	steps := args.Steps
	// pick a random neighbor
	randIndex := rand.Intn(len(ws.neighbors))
	neighbor := ws.neighbors[randIndex]
	if steps == 1 {
		reply.Server = neighbor
		reply.Err = OK
	} else {
		args := &RandomWalkArgs{}
		args.Steps = steps - 1
		var rpc_reply RandomWalkReply
		ok := call(neighbor, "WhanauServer.RandomWalk", args, &rpc_reply)
		if ok && (rpc_reply.Err == OK) {
			reply.Server = rpc_reply.Server
			reply.Err = OK
		}
	}

	return nil
}

// Gets the ID from node's local id table
func (ws *WhanauServer) GetId(args *GetIdArgs, reply *GetIdReply) error {
	layer := args.Layer
	//DPrintf("In getid, len(ws.ids): %d layer: %d", len(ws.ids), layer)
	// gets the id associated with a layer
	if 0 <= layer && layer < len(ws.ids) {
		id := ws.ids[layer]
		reply.Key = id
		reply.Err = OK
	}
	return nil
}

// Whanau Routing Protocal methods

// TODO
// Populates routing table
// nlayers = number of layers
// rf = size of finger table
// w = number of steps in random walk
// rd = size of database
// rs = number of nodes to collect samples from
// t = number of successors returned from sample per node
func (ws *WhanauServer) Setup(nlayers int, rf int, w int, rd int, rs int, t int) {
	DPrintf("In Setup of server %s", ws.myaddr)

	// fill up db by randomly sampling records from random walks
	// "The db table has the good property that each honest node’s stored records are frequently represented in other honest nodes’db tables"
	ws.db = ws.SampleRecords(rd, w)

	// reset ids, fingers, succ
	ws.ids = make([]KeyType, 0)
	ws.fingers = make([][]Finger, 0)
	ws.succ = make([][]Record, 0)
	for i := 0; i < nlayers; i++ {
		// populate tables in layers
		ws.ids = append(ws.ids, ws.ChooseID(i))
		//fmt.Printf("Finished ChooseID of server %s, layer %d\n", ws.myaddr, i)
		curFingerTable := ws.ConstructFingers(i, rf, w)
		ByFinger(FingerId).Sort(curFingerTable)
		ws.fingers = append(ws.fingers, curFingerTable)

		//fmt.Printf("Finished ConstructFingers of server %s, layer %d\n", ws.myaddr, i)
		curSuccessorTable := ws.Successors(i, w, rs, t)
		By(RecordKey).Sort(curSuccessorTable)
		ws.succ = append(ws.succ, curSuccessorTable)

		//fmt.Printf("Finished SuccessorTable of server %s, layer %d\n", ws.myaddr, i)
	}
}