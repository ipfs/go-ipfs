package hamt

import (
	"context"
	"time"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"
)

var log = logging.Logger("hamt")

// fetcher implements a background fetcher to retrieve missing child
// shards in large batches.  To allow streaming, it attempts to
// retrieve the missing shards in the order they will be read when
// traversing the tree depth first.
type fetcher struct {
	// note: the fields in this structure should only be accesses by
	// the 'mainLoop' go routine, all communication should be done via
	// channels

	ctx   context.Context
	dserv ipld.DAGService

	requestCh chan *Shard // channel for requesting the children of a shard
	resultCh  chan result // channel for retrieving the results of the request

	want *Shard // want is the job id (the parent shard) of the results we want next

	idle bool

	done chan batchJob // when a batch job completes results are sent to this channel

	todoFirst *job            // do this job first since we are waiting for its results
	todo      jobStack        // stack of jobs that still need to be done
	jobs      map[*Shard]*job // map of all jobs in which the results have not been collected yet

	// stats relevant for streaming the complete hamt directory
	doneCnt    int // job's done but results not yet retrieved
	hits       int // job's result already ready, no delay
	nearMisses int // job currently being worked on, small delay
	misses     int // job on todo stack but will be done in the next batch, larger delay

	// other useful stats
	cidCnt int

	start time.Time
}

// batchSize must be at least as large as the largest number of CIDs
// requested in a single job. For best performance it should likely be
// slightly larger as jobs are popped from the todo stack in order and a
// job close to the batchSize could force a very small batch to run.
// The recommend minimum size is thus a size slightly larger than the
// maximum number children in a HAMT node (which is the largest number
// of CIDs that could be requested in a single job) or 256 + 64 = 320
const batchSize = 320

//
// fetcher public interface
//

// startFetcher starts a new fetcher in the background
func startFetcher(ctx context.Context, dserv ipld.DAGService) *fetcher {
	log.Infof("fetcher: starting...")
	f := &fetcher{
		ctx:       ctx,
		dserv:     dserv,
		requestCh: make(chan *Shard),
		resultCh:  make(chan result),
		idle:      true,
		done:      make(chan batchJob),
		jobs:      make(map[*Shard]*job),
	}
	go f.mainLoop()
	return f
}

// result contains the result of a job, see getResult
type result struct {
	vals map[string]*Shard
	errs []error
}

// get gets the missing child shards for the hamt object.
// The missing children for the passed in shard are returned.  The
// children are then also retrieved in the background.  The result is
// the result of the batch request and not just the single job.  In
// particular, if the 'errs' field is empty the 'vals' of the result
// is guaranteed to contain all the missing child shards, but the
// map may also contain child shards of other jobs in the batch.
func (f *fetcher) get(hamt *Shard) result {
	f.requestCh <- hamt
	res := <-f.resultCh
	return res
}

//
// fetcher internals
//

type job struct {
	id  *Shard
	idx int /* index in the todo stack, an index of -1 means the job
	   is already done or being worked on now */
	cids []*cid.Cid
	res  result
}

func (f *fetcher) mainLoop() {
	f.start = time.Now()
	for {
		select {
		case id := <-f.requestCh:
			f.mainLoopHandleRequest(id)
		case bj := <-f.done:
			f.doneCnt += len(bj.jobs)
			f.cidCnt += len(bj.cids)
			f.launch()
			if f.want != nil {
				j := f.jobs[f.want]
				if j.res.vals != nil {
					f.mainLoopSendResult(j)
					f.want = nil
				}
			}
			log.Infof("fetcher: batch job done")
			f.mainLoopLogStats()
		case <-f.ctx.Done():
			if !f.idle {
				// wait unit batch job finishes
				<-f.done
			}
			log.Infof("fetcher: exiting")
			f.mainLoopLogStats()
			return
		}
	}
}

func (f *fetcher) mainLoopHandleRequest(id *Shard) {
	if f.want != nil {
		// programmer error
		panic("fetcher: can not request more than one result at a time")
	}
	j, ok := f.jobs[id]
	var err error
	if !ok {
		// job does not exist yet so add it
		j, err = f.mainLoopAddJob(id)
		if err != nil {
			f.resultCh <- result{errs: []error{err}}
			return
		}
		if j == nil {
			// no children that need to be retrieved
			f.resultCh <- result{vals: make(map[string]*Shard)}
			return
		}
		if f.idle {
			f.launch()
		}
	}
	if j.res.vals != nil {
		// job already completed so just send result
		f.hits++
		f.mainLoopSendResult(j)
		return
	}
	if j.idx != -1 {
		f.misses++
		// job is not currently running so
		// move job to todoFirst so that it will be done on the
		// next batch job
		f.todo.remove(j)
		f.todoFirst = j
	} else {
		// job already running
		f.nearMisses++
	}
	f.want = id
}

func (f *fetcher) mainLoopAddJob(hamt *Shard) (*job, error) {
	children, err := hamt.missingChildShards()
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, nil
	}
	j := &job{id: hamt, cids: children}
	if len(j.cids) > batchSize {
		// programmer error
		panic("job size larger than batchSize")
	}
	f.todo.push(j)
	f.jobs[j.id] = j
	return j, nil
}

func (f *fetcher) mainLoopSendResult(j *job) {
	f.resultCh <- j.res
	delete(f.jobs, j.id)
	f.doneCnt--
	if len(j.res.errs) != 0 {
		return
	}
	// we want the first child to be the first to run not the last so
	// add the jobs in the reverse order
	for i := len(j.cids) - 1; i >= 0; i-- {
		hamt := j.res.vals[string(j.cids[i].Bytes())]
		f.mainLoopAddJob(hamt)
	}
	if f.idle {
		f.launch()
	}
}

func (f *fetcher) mainLoopLogStats() {
	log.Infof("fetcher stats (cids, done, hits, nearMisses, misses): %d %d %d %d %d", f.cidCnt, f.doneCnt, f.hits, f.nearMisses, f.misses)
	elapsed := time.Now().Sub(f.start).Seconds()
	log.Infof("fetcher perf (cids/sec, jobs/sec) %f %f", float64(f.cidCnt)/elapsed, float64(f.doneCnt+f.hits+f.nearMisses+f.misses)/elapsed)
}

type batchJob struct {
	cids []*cid.Cid
	jobs []*job
}

func (b *batchJob) add(j *job) {
	b.cids = append(b.cids, j.cids...)
	b.jobs = append(b.jobs, j)
	j.idx = -1
}

func (f *fetcher) launch() {
	bj := batchJob{}

	// always do todoFirst
	if f.todoFirst != nil {
		bj.add(f.todoFirst)
		f.todoFirst = nil
	}

	// pop requets from todo list until we hit the batchSize
	for !f.todo.empty() && len(bj.cids)+len(f.todo.top().cids) <= batchSize {
		j := f.todo.pop()
		bj.add(j)
	}

	if len(bj.cids) == 0 {
		if !f.idle {
			log.Infof("fetcher: entering idle state: no more jobs")
		}
		f.idle = true
		return
	}

	// launch job
	log.Infof("fetcher: starting batch job, size = %d", len(bj.cids))
	f.idle = false
	go func() {
		ch := f.dserv.GetMany(f.ctx, bj.cids)
		fetched := result{vals: make(map[string]*Shard)}
		for no := range ch {
			if no.Err != nil {
				fetched.errs = append(fetched.errs, no.Err)
				continue
			}
			hamt, err := NewHamtFromDag(f.dserv, no.Node)
			if err != nil {
				fetched.errs = append(fetched.errs, err)
				continue
			}
			fetched.vals[string(no.Node.Cid().Bytes())] = hamt
		}
		for _, job := range bj.jobs {
			job.res = fetched
		}
		f.done <- bj
	}()
}

// jobStack is a specialized stack implementation.  It has the
// property that once an item is added to the stack its position will
// never change so it can be referenced by index
type jobStack struct {
	c []*job
}

func (js *jobStack) empty() bool {
	return len(js.c) == 0
}

func (js *jobStack) top() *job {
	return js.c[len(js.c)-1]
}

func (js *jobStack) push(j *job) {
	j.idx = len(js.c)
	js.c = append(js.c, j)
}

func (js *jobStack) pop() *job {
	j := js.top()
	js.remove(j)
	return j
}

// remove marks a job as empty and attempts to remove it if it is at
// the top of a stack
func (js *jobStack) remove(j *job) {
	js.c[j.idx] = nil
	j.idx = -1
	js.popEmpty()
}

// popEmpty removes all empty jobs at the top of the stack
func (js *jobStack) popEmpty() {
	for len(js.c) > 0 && js.c[len(js.c)-1] == nil {
		js.c = js.c[:len(js.c)-1]
	}
}
