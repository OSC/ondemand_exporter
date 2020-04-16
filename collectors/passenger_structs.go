// MIT License
//
// Copyright (c) 2020 Ohio Supercomputer Center
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package collectors

type PassengerInfo struct {
	SuperGroups []PassengerSuperGroup `xml:"supergroups>supergroup"`
}

type PassengerSuperGroup struct {
	Group PassengerGroup `xml:"group"`
}

type PassengerGroup struct {
	AppRoot   string             `xml:"app_root"`
	Processes []PassengerProcess `xml:"processes>process"`
}

type PassengerProcess struct {
	RSS               int   `xml:"rss"`
	CPU               int   `xml:"cpu"`
	RealMemory        int   `xml:"real_memory"`
	RequestsProcessed int   `xml:"processed"`
	SpawnStartTime    int64 `xml:"spawn_start_time"`
}
