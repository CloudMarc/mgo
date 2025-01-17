// mgo - MongoDB driver for Go
// 
// Copyright (c) 2010-2011 - Gustavo Niemeyer <gustavo@niemeyer.net>
// 
// All rights reserved.
// 
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
// 
//     * Redistributions of source code must retain the above copyright notice,
//       this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above copyright notice,
//       this list of conditions and the following disclaimer in the documentation
//       and/or other materials provided with the distribution.
//     * Neither the name of the copyright holder nor the names of its
//       contributors may be used to endorse or promote products derived from
//       this software without specific prior written permission.
// 
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR
// CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package mgo

import (
	//"launchpad.net/gobson/bson"
	"github.com/CloudMarc/mgo/gobson"
	"sync"
	"net"
	"os"
)

type replyFunc func(err os.Error, reply *replyOp, docNum int, docData []byte)

type mongoSocket struct {
	sync.Mutex
	server        *mongoServer // nil when cached
	conn          *net.TCPConn
	addr          string // For debugging only.
	nextRequestId uint32
	replyFuncs    map[uint32]replyFunc
	references    int
	auth          []authInfo
	logout        []authInfo
	cachedNonce   string
	gotNonce      sync.Cond
	dead          os.Error
}

type queryOp struct {
	collection string
	query      interface{}
	skip       int32
	limit      int32
	selector   interface{}
	flags      uint32
	replyFunc  replyFunc
}

type getMoreOp struct {
	collection string
	limit      int32
	cursorId   int64
	replyFunc  replyFunc
}

type replyOp struct {
	flags     uint32
	cursorId  int64
	firstDoc  int32
	replyDocs int32
}

type insertOp struct {
	collection string        // "database.collection"
	documents  []interface{} // One or more documents to insert
}

type updateOp struct {
	collection string // "database.collection"
	selector   interface{}
	update     interface{}
	flags      uint32
}

type deleteOp struct {
	collection string // "database.collection"
	selector   interface{}
	flags      uint32
}

type requestInfo struct {
	bufferPos int
	replyFunc replyFunc
}

func newSocket(server *mongoServer, conn *net.TCPConn) *mongoSocket {
	socket := &mongoSocket{conn: conn, addr: server.Addr}
	socket.gotNonce.L = &socket.Mutex
	socket.replyFuncs = make(map[uint32]replyFunc)
	socket.Acquired(server)
	stats.socketsAlive(+1)
	debugf("Socket %p to %s: initialized", socket, socket.addr)
	socket.resetNonce()
	go socket.readLoop()
	return socket
}

// Inform the socket it's being put in use, either right after a
// connection or after being recycled.
func (socket *mongoSocket) Acquired(server *mongoServer) os.Error {
	socket.Lock()
	if socket.server != nil {
		panic("Attempting to reacquire an owned socket.")
	}
	if socket.dead != nil {
		socket.Unlock()
		return socket.dead
	}
	if socket.references > 0 {
		panic("Socket acquired out of cache with references")
	}
	socket.server = server
	socket.references++
	stats.socketsInUse(+1)
	stats.socketRefs(+1)
	socket.Unlock()
	return nil
}

// Acquire the socket again, increasing its refcount.  The socket
// will only be recycled when it's released as many times as it's
// acquired.
func (socket *mongoSocket) Acquire() (isMaster bool) {
	socket.Lock()
	if socket.references == 0 {
		panic("socket.Acquire() with references == 0")
	}
	socket.references++
	stats.socketRefs(+1)
	isMaster = socket.server.IsMaster()
	socket.Unlock()
	return isMaster
}

// Decrement the socket refcount. The socket will be recycled once its
// released as many times as it's acquired.
func (socket *mongoSocket) Release() {
	socket.Lock()
	if socket.references == 0 {
		panic("socket.Release() with references == 0")
	}
	socket.references--
	stats.socketRefs(-1)
	if socket.references == 0 {
		stats.socketsInUse(-1)
		server := socket.server
		socket.server = nil
		socket.Unlock()
		socket.LogoutAll()
		server.RecycleSocket(socket)
	} else {
		socket.Unlock()
	}
}

// Close terminates the socket use.
func (socket *mongoSocket) Close() {
	socket.kill(os.NewError("Closed explicitly"))
}

func (socket *mongoSocket) kill(err os.Error) {
	socket.Lock()
	if socket.dead != nil {
		debugf("Socket %p to %s: killed again: %s (previously: %s)", socket, socket.addr, err.String(), socket.dead.String())
		socket.Unlock()
		return
	}
	logf("Socket %p to %s: closing: %s", socket, socket.addr, err.String())
	socket.dead = err
	socket.conn.Close()
	stats.socketsAlive(-1)
	replyFuncs := socket.replyFuncs
	socket.replyFuncs = make(map[uint32]replyFunc)
	socket.Unlock()
	for _, f := range replyFuncs {
		logf("Socket %p to %s: notifying replyFunc of closed socket: %s", socket, socket.addr, err.String())
		f(err, nil, -1, nil)
	}
}

func (socket *mongoSocket) SimpleQuery(op *queryOp) (data []byte, err os.Error) {
	var mutex sync.Mutex
	var replyData []byte
	var replyErr os.Error
	mutex.Lock()
	op.replyFunc = func(err os.Error, reply *replyOp, docNum int, docData []byte) {
		replyData = docData
		replyErr = err
		mutex.Unlock()
	}
	err = socket.Query(op)
	if err != nil {
		return nil, err
	}
	mutex.Lock() // Wait.
	if replyErr != nil {
		return nil, replyErr
	}
	return replyData, nil
}

func (socket *mongoSocket) Query(ops ...interface{}) (err os.Error) {

	if lops := socket.flushLogout(); len(lops) > 0 {
		ops = append(lops, ops...)
	}

	buf := make([]byte, 0, 256)

	// Serialize operations synchronously to avoid interrupting
	// other goroutines while we can't really be sending data.
	// Also, record id positions so that we can compute request
	// ids at once later with the lock already held.
	requests := make([]requestInfo, len(ops))
	requestCount := 0

	for _, op := range ops {
		debugf("Socket %p to %s: serializing op: %#v", socket, socket.addr, op)
		start := len(buf)
		var replyFunc replyFunc
		switch op := op.(type) {

		case *updateOp:
			buf = addHeader(buf, 2001)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, int32(op.flags))
			debugf("Socket %p to %s: serializing selector document: %#v", socket, socket.addr, op.selector)
			buf, err = addBSON(buf, op.selector)
			if err != nil {
				return err
			}
			debugf("Socket %p to %s: serializing update document: %#v", socket, socket.addr, op.update)
			buf, err = addBSON(buf, op.update)
			if err != nil {
				return err
			}

		case *insertOp:
			buf = addHeader(buf, 2002)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.collection)
			for _, doc := range op.documents {
				debugf("Socket %p to %s: serializing document for insertion: %#v", socket, socket.addr, doc)
				buf, err = addBSON(buf, doc)
				if err != nil {
					return err
				}
			}

		case *queryOp:
			buf = addHeader(buf, 2004)
			buf = addInt32(buf, int32(op.flags))
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, op.skip)
			buf = addInt32(buf, op.limit)
			buf, err = addBSON(buf, op.query)
			if err != nil {
				return err
			}
			if op.selector != nil {
				buf, err = addBSON(buf, op.selector)
				if err != nil {
					return err
				}
			}
			replyFunc = op.replyFunc

		case *getMoreOp:
			buf = addHeader(buf, 2005)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, op.limit)
			buf = addInt64(buf, op.cursorId)
			replyFunc = op.replyFunc

		case *deleteOp:
			buf = addHeader(buf, 2006)
			buf = addInt32(buf, 0) // Reserved
			buf = addCString(buf, op.collection)
			buf = addInt32(buf, int32(op.flags))
			debugf("Socket %p to %s: serializing selector document: %#v", socket, socket.addr, op.selector)
			buf, err = addBSON(buf, op.selector)
			if err != nil {
				return err
			}

		default:
			panic("Internal error: unknown operation type")
		}

		setInt32(buf, start, int32(len(buf)-start))

		if replyFunc != nil {
			request := &requests[requestCount]
			request.replyFunc = replyFunc
			request.bufferPos = start
			requestCount++
		}
	}

	// Buffer is ready for the pipe.  Lock, allocate ids, and enqueue.

	socket.Lock()
	if socket.dead != nil {
		socket.Unlock()
		debug("Socket %p to %s: failing query, already closed: %s", socket, socket.addr, socket.dead.String())
		// XXX This seems necessary in case the session is closed concurrently
		// with a query being performed, but it's not yet tested:
		for i := 0; i != requestCount; i++ {
			request := &requests[i]
			if request.replyFunc != nil {
				request.replyFunc(socket.dead, nil, -1, nil)
			}
		}
		return socket.dead
	}

	// Reserve id 0 for requests which should have no responses.
	requestId := socket.nextRequestId + 1
	if requestId == 0 {
		requestId++
	}
	socket.nextRequestId = requestId + uint32(requestCount)
	for i := 0; i != requestCount; i++ {
		request := &requests[i]
		setInt32(buf, request.bufferPos+4, int32(requestId))
		socket.replyFuncs[requestId] = request.replyFunc
		requestId++
	}

	debugf("Socket %p to %s: sending %d op(s) (%d bytes)", socket, socket.addr, len(ops), len(buf))
	stats.sentOps(len(ops))

	_, err = socket.conn.Write(buf)
	socket.Unlock()
	return err
}

func fill(r *net.TCPConn, b []byte) os.Error {
	l := len(b)
	n, err := r.Read(b)
	for n != l && err == nil {
		var ni int
		ni, err = r.Read(b[n:])
		n += ni
	}
	return err
}

// Estimated minimum cost per socket: 1 goroutine + memory for the largest
// document ever seen.
func (socket *mongoSocket) readLoop() {
	p := [36]byte{}[:] // 16 from header + 20 from OP_REPLY fixed fields
	s := [4]byte{}[:]
	conn := socket.conn // No locking, conn never changes.
	for {
		// XXX Handle timeouts, , etc
		err := fill(conn, p)
		if err != nil {
			socket.kill(err)
			return
		}

		totalLen := getInt32(p, 0)
		responseTo := getInt32(p, 8)
		opCode := getInt32(p, 12)

		// Don't use socket.server.Addr here.  socket is not locked.
		debugf("Socket %p to %s: got reply (%d bytes)", socket, socket.addr, totalLen)

		_ = totalLen

		if opCode != 1 {
			socket.kill(os.NewError("opcode != 1, corrupted data?"))
			return
		}

		reply := replyOp{
			flags:     uint32(getInt32(p, 16)),
			cursorId:  getInt64(p, 20),
			firstDoc:  getInt32(p, 28),
			replyDocs: getInt32(p, 32),
		}

		stats.receivedOps(+1)
		stats.receivedDocs(int(reply.replyDocs))

		socket.Lock()
		replyFunc, replyFuncFound := socket.replyFuncs[uint32(responseTo)]
		socket.Unlock()

		if replyFunc != nil && reply.replyDocs == 0 {
			replyFunc(nil, &reply, -1, nil)
		} else {
			for i := 0; i != int(reply.replyDocs); i++ {
				err := fill(conn, s)
				if err != nil {
					socket.kill(err)
					return
				}

				b := make([]byte, int(getInt32(s, 0)))

				// copy(b, s) in an efficient way.
				b[0] = s[0]
				b[1] = s[1]
				b[2] = s[2]
				b[3] = s[3]

				err = fill(conn, b[4:])
				if err != nil {
					socket.kill(err)
					return
				}

				if globalDebug && globalLogger != nil {
					m := bson.M{}
					if err := bson.Unmarshal(b, m); err == nil {
						debugf("Socket %p to %s: received document: %#v", socket, socket.addr, m)
					}
				}

				if replyFunc != nil {
					replyFunc(nil, &reply, i, b)
				}

				// XXX Do bound checking against totalLen.
			}
		}

		// Only remove replyFunc after iteration, so that kill() will see it.
		socket.Lock()
		if replyFuncFound {
			socket.replyFuncs[uint32(responseTo)] = replyFunc, false
		}
		socket.Unlock()

		// XXX Do bound checking against totalLen.
	}
}

var emptyHeader = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func addHeader(b []byte, opcode int) []byte {
	i := len(b)
	b = append(b, emptyHeader...)
	// Enough for current opcodes.
	b[i+12] = byte(opcode)
	b[i+13] = byte(opcode >> 8)
	return b
}

func addInt32(b []byte, i int32) []byte {
	return append(b, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
}

func addInt64(b []byte, i int64) []byte {
	return append(b, byte(i), byte(i>>8), byte(i>>16), byte(i>>24),
		byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
}

func addCString(b []byte, s string) []byte {
	b = append(b, []byte(s)...)
	b = append(b, 0)
	return b
}

func addBSON(b []byte, doc interface{}) ([]byte, os.Error) {
	data, err := bson.Marshal(doc)
	if err != nil {
		return b, err
	}
	return append(b, data...), nil
}

func setInt32(b []byte, pos int, i int32) {
	b[pos] = byte(i)
	b[pos+1] = byte(i >> 8)
	b[pos+2] = byte(i >> 16)
	b[pos+3] = byte(i >> 24)
}

func getInt32(b []byte, pos int) int32 {
	return (int32(b[pos+0])) |
		(int32(b[pos+1]) << 8) |
		(int32(b[pos+2]) << 16) |
		(int32(b[pos+3]) << 24)
}

func getInt64(b []byte, pos int) int64 {
	return (int64(b[pos+0])) |
		(int64(b[pos+1]) << 8) |
		(int64(b[pos+2]) << 16) |
		(int64(b[pos+3]) << 24) |
		(int64(b[pos+4]) << 32) |
		(int64(b[pos+5]) << 40) |
		(int64(b[pos+6]) << 48) |
		(int64(b[pos+7]) << 56)
}
