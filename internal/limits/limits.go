/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Package limit provides a module object that can be used to restrict the
// concurrency and rate of the messages flow globally or on per-source,
// per-destination basis.
//
// Note, all domain inputs are interpreted with the assumption they are already
// normalized.
//
// Low-level components are available in the limiters/ subpackage.
package limits

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/limits/limiters"
)

type Group struct {
	instName string

	global limiters.MultiLimit
	ip     *limiters.BucketSet // BucketSet of MultiLimit
	source *limiters.BucketSet // BucketSet of MultiLimit
	dest   *limiters.BucketSet // BucketSet of MultiLimit
}

func New(_, instName string, _, _ []string) (module.Module, error) {
	return &Group{
		instName: instName,
	}, nil
}

func (g *Group) Init(cfg *config.Map) error {
	var (
		globalL []limiters.L
		ipL     []func() limiters.L
		sourceL []func() limiters.L
		destL   []func() limiters.L
	)

	for _, child := range cfg.Block.Children {
		if len(child.Args) < 1 {
			return config.NodeErr(child, "at least two arguments are required")
		}

		var (
			ctor func() limiters.L
			err  error
		)
		switch kind := child.Args[0]; kind {
		case "rate":
			ctor, err = rateCtor(child, child.Args[1:])
		case "concurrency":
			ctor, err = concurrencyCtor(child, child.Args[1:])
		default:
			return config.NodeErr(child, "unknown limit kind: %v", kind)
		}
		if err != nil {
			return err
		}

		switch scope := child.Name; scope {
		case "all":
			globalL = append(globalL, ctor())
		case "ip":
			ipL = append(ipL, ctor)
		case "source":
			sourceL = append(sourceL, ctor)
		case "destination":
			destL = append(destL, ctor)
		default:
			return config.NodeErr(child, "unknown limit scope: %v", scope)
		}
	}

	// 20010 is slightly higher than the default max. recipients count in
	// endpoint/smtp.
	g.global = limiters.MultiLimit{Wrapped: globalL}
	if len(ipL) != 0 {
		g.ip = limiters.NewBucketSet(func() limiters.L {
			l := make([]limiters.L, 0, len(ipL))
			for _, ctor := range ipL {
				l = append(l, ctor())
			}
			return &limiters.MultiLimit{Wrapped: l}
		}, 1*time.Minute, 20010)
	}
	if len(sourceL) != 0 {
		g.source = limiters.NewBucketSet(func() limiters.L {
			l := make([]limiters.L, 0, len(sourceL))
			for _, ctor := range sourceL {
				l = append(l, ctor())
			}
			return &limiters.MultiLimit{Wrapped: l}
		}, 1*time.Minute, 20010)
	}
	if len(destL) != 0 {
		g.dest = limiters.NewBucketSet(func() limiters.L {
			l := make([]limiters.L, 0, len(sourceL))
			for _, ctor := range sourceL {
				l = append(l, ctor())
			}
			return &limiters.MultiLimit{Wrapped: l}
		}, 1*time.Minute, 20010)
	}

	return nil
}

func rateCtor(node config.Node, args []string) (func() limiters.L, error) {
	period := 1 * time.Second
	burst := 0

	switch len(args) {
	case 2:
		var err error
		period, err = time.ParseDuration(args[1])
		if err != nil {
			return nil, config.NodeErr(node, "%v", err)
		}
		fallthrough
	case 1:
		var err error
		burst, err = strconv.Atoi(args[0])
		if err != nil {
			return nil, config.NodeErr(node, "%v", err)
		}
	case 0:
		return nil, config.NodeErr(node, "at least burst size is needed")
	default:
		return nil, config.NodeErr(node, "too many arguments")
	}

	return func() limiters.L {
		return limiters.NewRate(burst, period)
	}, nil
}

func concurrencyCtor(node config.Node, args []string) (func() limiters.L, error) {
	if len(args) != 1 {
		return nil, config.NodeErr(node, "max concurrency value is needed")
	}
	max, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, config.NodeErr(node, "%v", err)
	}
	return func() limiters.L {
		return limiters.NewSemaphore(max)
	}, nil
}

func (g *Group) TakeMsg(ctx context.Context, addr net.IP, sourceDomain string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := g.global.TakeContext(ctx); err != nil {
		return err
	}

	if g.ip != nil {
		if err := g.ip.TakeContext(ctx, addr.String()); err != nil {
			g.global.Release()
			return err
		}
	}
	if g.source != nil {
		if err := g.source.TakeContext(ctx, sourceDomain); err != nil {
			g.global.Release()
			g.ip.Release(addr.String())
			return err
		}
	}
	return nil
}

func (g *Group) TakeDest(ctx context.Context, domain string) error {
	if g.dest == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return g.dest.TakeContext(ctx, domain)
}

func (g *Group) ReleaseMsg(addr net.IP, sourceDomain string) {
	g.global.Release()
	if g.ip != nil {
		g.ip.Release(addr.String())
	}
	if g.source != nil {
		g.source.Release(sourceDomain)
	}
}

func (g *Group) ReleaseDest(domain string) {
	if g.dest == nil {
		return
	}
	g.dest.Release(domain)
}

func (g *Group) Name() string {
	return "limits"
}

func (g *Group) InstanceName() string {
	return g.instName
}

func init() {
	module.Register("limits", New)
}
