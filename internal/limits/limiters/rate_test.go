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

package limiters

import (
	"context"
	"testing"
	"time"
)

func TestRate_TakeContext(t *testing.T) {
	type ctrArgs struct {
		burstSize int
		interval  time.Duration
	}
	type args struct {
		ctx context.Context
	}
	type loop struct {
		count int
		sleep time.Duration
	}
	tests := []struct {
		name            string
		ctrArgs         ctrArgs
		args            args
		loop            loop
		wantErr         bool
		totalTimeAbove  time.Duration
		totalTimeBefore time.Duration
		close           bool
	}{
		{
			name:            "rate all good",
			ctrArgs:         ctrArgs{burstSize: 1, interval: 10 * time.Millisecond},
			args:            args{ctx: context.Background()},
			loop:            loop{count: 20},
			wantErr:         false,
			totalTimeAbove:  19 * 10 * time.Millisecond, // 19 because of burst 1
			totalTimeBefore: 20 * 10 * time.Millisecond, // it should be well below 200ms even on very slow machines
		},
		{
			name:            "rate burst 0",
			ctrArgs:         ctrArgs{burstSize: 0, interval: 10 * time.Second},
			args:            args{ctx: context.Background()},
			loop:            loop{count: 20},
			wantErr:         false,
			totalTimeBefore: 10 * time.Millisecond, // make sure to give enough time on very very slow machines
		},
		{
			name:            "rate closed",
			ctrArgs:         ctrArgs{burstSize: 0, interval: 10 * time.Second},
			args:            args{ctx: context.Background()},
			loop:            loop{count: 20},
			wantErr:         true,
			close:           true,
			totalTimeBefore: 10 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRate(tt.ctrArgs.burstSize, tt.ctrArgs.interval)
			if tt.close {
				r.Close()
			}
			start := time.Now()
			for i := 0; i < tt.loop.count; i++ {
				if err := r.TakeContext(tt.args.ctx); (err != nil) != tt.wantErr {
					t.Errorf("Rate.TakeContext() error = %v, wantErr %v", err, tt.wantErr)
				}
				time.Sleep(tt.loop.sleep)
			}
			endTime := time.Since(start)
			if endTime < tt.totalTimeAbove {
				t.Errorf("Rate.TakeContext() took not enough time, want %s, got %s", tt.totalTimeAbove, endTime)
			}
			if endTime > tt.totalTimeBefore {
				t.Errorf("Rate.TakeContext() took too much time, want %s, got %s", tt.totalTimeBefore, endTime)
			}
		})
	}
}
