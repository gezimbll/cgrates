/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>
*/

package main

import (
	"strings"
	"testing"

	"github.com/cgrates/cgrates/utils"
	tea "github.com/charmbracelet/bubbletea"
)

type testClient struct{}

func (testClient) Methods() []string { return nil }
func (testClient) Describe(string) *utils.MethodDescriptor {
	return nil
}
func (testClient) Call(string, any) (any, error) { return nil, nil }

func TestResultViewportScrollsHorizontally(t *testing.T) {
	m := newModel(testClient{}, "")
	m.result.Width, m.result.Height = 8, 1
	m.result.SetContent("0123456789abcdef")
	before := m.result.View()
	next, _ := m.updateResult(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(model)
	if after := m.result.View(); after == before {
		t.Fatalf("right did not scroll %q", after)
	}
}

func TestResizeClampsResultViewport(t *testing.T) {
	m := newModel(testClient{}, "")
	m.result.Width, m.result.Height = 40, 5
	m.result.SetContent(strings.Repeat("line\n", 100))
	m.result.GotoBottom()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 50})
	m = next.(model)
	if m.result.PastBottom() {
		t.Fatalf("offset %d is past bottom after resize", m.result.YOffset)
	}
	if m.result.Height != 48 {
		t.Fatalf("height = %d, want 48", m.result.Height)
	}
}
