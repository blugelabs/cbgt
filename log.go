//  Copyright (c) 2020 The Bluge Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//              http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cbgt

import (
	"io"
	"log"
)

type Log interface {

	Print(args ...interface{})
	Printf(format string, args ...interface{})

	Error(err error) error
	Errorf(format string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Trace(args ...interface{})
	Tracef(format string, args ...interface{})
}

type StdLibLog log.Logger

func NewStdLibLog(out io.Writer, prefix string, flag int) *StdLibLog {
	l := log.New(out, prefix, flag)
	sll := StdLibLog(*l)
	return &sll
}

func (s *StdLibLog) Print(args ...interface{}) {
	(*log.Logger)(s).Print(args...)
}

func (s *StdLibLog) Printf(format string, args ...interface{}) {
	(*log.Logger)(s).Printf(format, args...)
}

func (s *StdLibLog) Error(err error) error {
	(*log.Logger)(s).Print(err)
	return err
}

func (s *StdLibLog) Errorf(format string, args ...interface{}) {
	(*log.Logger)(s).Printf(format, args...)
}

func (s *StdLibLog) Warn(args ...interface{}) {
	(*log.Logger)(s).Print(args...)
}

func (s *StdLibLog) Warnf(format string, args ...interface{}) {
	(*log.Logger)(s).Printf(format, args...)
}

func (s *StdLibLog) Debug(args ...interface{}) {
	(*log.Logger)(s).Print(args...)
}

func (s *StdLibLog) Debugf(format string, args ...interface{}) {
	(*log.Logger)(s).Printf(format, args...)
}

func (s *StdLibLog) Trace(args ...interface{}) {
	(*log.Logger)(s).Print(args...)
}

func (s *StdLibLog) Tracef(format string, args ...interface{}) {
	(*log.Logger)(s).Printf(format, args...)
}