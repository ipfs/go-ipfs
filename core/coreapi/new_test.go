package coreapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/tests"
)

var errSkip = errors.New("skip")

type makeSingle func() *CoreAPI

func (f makeSingle) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]iface.CoreAPI, error) {
	if n != 1 {
		return nil, errSkip
	}

	return []iface.CoreAPI{
		f(),
	}, nil
}

func BasicTests(ts *tests.TestSuite, t *testing.T) {
	ts.TestKey(t)
	//ts.TestUnixfs(t)
}

func TestNew(t *testing.T) {
	mn := mocknet.New(context.Background())

	for _, testcase := range []struct {
		name string
		opts []Option
		test func(*tests.TestSuite, *testing.T)
	}{
		{
			name: "default",
			test: BasicTests,
		},
		{
			name: "online",
			test: BasicTests,
			opts: []Option{
				Online(),
				Override(Libp2pHost, libp2p.MockHost),
				Provide(func() mocknet.Mocknet { return mn }),
			},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			f := func() *CoreAPI {
				api, err := New(testcase.opts...)
				if err != nil {
					t.Fatal(err)
				}

				if api == nil {
					t.Fatal("api is nil")
				}

				return api
			}

			ts := &tests.TestSuite{
				Provider: makeSingle(f),
			}

			testcase.test(ts, t)
		})
	}
}

type testif interface {
	test()
}

type testst struct {
	i int
}

func (s *testst) test() {
	s.i++
}

func TestAs(t *testing.T) {
	tstr := &testst{}

	var out struct {
		T testif
	}

	// as(struct)

	app := fx.New(fx.Provide(as(tstr, new(testif))), fx.Extract(&out))
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	out.T.test()
	if tstr.i != 1 {
		t.Error("unexpected tstr.i")
	}

	// as(func()struct)

	tstr = &testst{}
	ctor1 := func() *testst { return tstr }
	app = fx.New(fx.Provide(as(ctor1, new(testif))), fx.Extract(&out))
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	out.T.test()
	if tstr.i != 1 {
		t.Error("unexpected tstr.i")
	}

	// as(func(in)(struct, error))

	tstr = &testst{}
	ctor2 := func(testStr string) (*testst, error) {
		if testStr != "test" {
			t.Fatal(testStr)
		}
		return tstr, nil
	}
	app = fx.New(
		fx.Provide(as(ctor2, new(testif))),
		fx.Provide(func() string { return "test" }),
		fx.Extract(&out))
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	out.T.test()
	if tstr.i != 1 {
		t.Error("unexpected tstr.i")
	}

	tstr = &testst{}
	ctor3 := func(testStr string) (*testst, error) {
		return nil, errors.New(testStr)
	}
	app = fx.New(
		fx.Provide(as(ctor3, new(testif))),
		fx.Provide(func() string { return "toast" }),
		fx.Extract(&out))
	if err := app.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "toast") {
		t.Fatal(err)
	}

	if tstr.i != 0 {
		t.Error("unexpected tstr.i")
	}

	// out arg check

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	as(func() int { return 0 }, "potato")
}

var _ tests.Provider = makeSingle(nil)
