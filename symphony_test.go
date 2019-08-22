/*
 Copyright (c) 2012-2019 Grab Taxi Holdings PTE LTD (GRAB), All Rights Reserved. NOTICE: All information contained herein
 is, and remains the property of GRAB. The intellectual and technical concepts contained herein are confidential, proprietary
 and controlled by GRAB and may be covered by patents, patents in process, and are protected by trade secret or copyright law.

 You are strictly forbidden to copy, download, store (in any medium), transmit, disseminate, adapt or change this material
 in any way unless prior written permission is obtained from GRAB. Access to the source code contained herein is hereby
 forbidden to anyone except current GRAB employees or contractors with binding Confidentiality and Non-disclosure agreements
 explicitly covering such access.

 The copyright notice above does not evidence any actual or intended publication or disclosure of this source code,
 which includes information that is confidential and/or proprietary, and is a trade secret, of GRAB.

 ANY REPRODUCTION, MODIFICATION, DISTRIBUTION, PUBLIC PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE
 CODE WITHOUT THE EXPRESS WRITTEN CONSENT OF GRAB IS STRICTLY PROHIBITED, AND IN VIOLATION OF APPLICABLE LAWS AND
 INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY
 OR IMPLY ANY RIGHTS TO REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL ANYTHING
 THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
*/

package symphony

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	symphony := New()
	assert.NotNil(t, symphony, "symphony cannot be nil")
}

func TestNoExisted(t *testing.T) {
	symphony := New()
	symphony.Add("test", []string{"dep1"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Equal(t, `error: Function "dep1" not exists`, err.Error())
}

func TestSelfDep(t *testing.T) {
	symphony := New()
	symphony.Add("test", []string{"test"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Equal(t, `error: Function "test" depends of itself`, err.Error())
}

func TestDupName(t *testing.T) {
	symphony := New()
	symphony.Add("test", []string{"test"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("test", []string{"test"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Contains(t, err.Error(), "error: Duplicated Func Name")
}

func TestCyclicDep1(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{"f2"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f2", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f3", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Contains(t, err.Error(), "error: Has cyclic dependency")
}

func TestCyclicDep2(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{"f2", "f3"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f2", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f3", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Contains(t, err.Error(), "error: Has cyclic dependency")
}

func TestCyclicDep3(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{"f2"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f2", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.Contains(t, err.Error(), "error: Has cyclic dependency")
}

// pCtx stops earlier (100ms) than symphony.Do timeout (1000ms)
func TestDoTimeoutExternal(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{"f2", "f3"}, func(res map[string]*TaskState) (interface{}, error) {
		time.Sleep(time.Millisecond * 1000)
		return "test result", nil
	})
	symphony.Add("f2", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f3", nil, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	ctxRecovery, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	var err error
	pCtx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	go func() {
		_, err = symphony.Do(pCtx, 1000)
		cancel()
	}()

	select {
	case <-ctxRecovery.Done():
		assert.NotNil(t, pCtx.Err(), "pCtx should already be cancelled, err: %s", pCtx.Err())
		assert.NotNil(t, err, "timeout, and err should be nil")
		assert.Contains(t, err.Error(), "error: Timeout")
	}
}

// pCtx stops after (1000ms) than symphony.Do timeout (100ms)
func TestDoTimeoutInternal(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{"f2", "f3"}, func(res map[string]*TaskState) (interface{}, error) {
		time.Sleep(time.Millisecond * 1000)
		return "test result", nil
	})
	symphony.Add("f2", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	symphony.Add("f3", nil, func(res map[string]*TaskState) (interface{}, error) {
		return "test result", nil
	})
	ctxRecovery, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	var err error
	pCtx, _ := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	go func() {
		_, err = symphony.Do(pCtx, 100)
		cancel()
	}()

	select {
	case <-ctxRecovery.Done():
		assert.Nil(t, pCtx.Err(), "pCtx has not cancelled, err should be nil")
		assert.NotNil(t, err, "timeout, and err should be nil")
		assert.Contains(t, err.Error(), "error: Timeout")
	}
}

func TestDoIncorrectResult(t *testing.T) {
	var shouldBeFalse = false
	symphony := New()
	symphony.Add("f1", []string{}, func(res map[string]*TaskState) (interface{}, error) {
		time.Sleep(time.Second * 1)
		shouldBeFalse = true
		return "f1 result", nil
	})
	symphony.Add("f2", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		shouldBeFalse = false
		return "f2 result", nil
	})
	_, err := symphony.Do(context.Background(), 10000)

	assert.True(t, err == nil && !shouldBeFalse, "Incorrect goroutines execution order")
}

func TestDoResult(t *testing.T) {
	symphony := New()
	symphony.Add("f1", []string{}, func(res map[string]*TaskState) (interface{}, error) {
		time.Sleep(time.Second * 1)
		return "f1 result", nil
	})
	symphony.Add("f2", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		return "f2 result", nil
	})
	res, err := symphony.Do(context.Background(), 10000)

	f1Result, f1 := res["f1"]
	f2Result, f2 := res["f2"]

	assert.True(t, err == nil && f1 && f2 && f1Result.R == "f1 result" && f2Result.R == "f2 result", "Incorrect goroutines execution result, f1: %s, f2: %s", f1Result, f2Result)
}

func TestDoMoreResult(t *testing.T) {
	symphony := New()

	symphony.Add("f2", []string{"f1", "f4"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f2==")
		return "f2 result", nil
	})
	symphony.Add("f3", []string{"f2", "f4"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f3==")
		return fmt.Sprintf("%s|%s|%s", res["f2"].R, res["f4"].R, "f3 result"), nil
	})
	symphony.Add("f4", []string{"f5"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f4==")
		return "f4 result", nil
	})
	symphony.Add("f5", nil, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f5==")
		return "f5 result", nil
	})
	symphony.Add("f6", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f6==")
		return fmt.Sprintf("%s|%s", res["f3"].R, "f6 result"), nil
	})
	symphony.Add("f7", []string{"f4"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f7==")
		return "f7 result", nil
	})
	symphony.Add("f1", nil, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f1==")
		return "f1 result", nil
	})
	res, err := symphony.Do(context.Background(), 10000)

	f1r, f1 := res["f1"]
	f2r, f2 := res["f2"]
	f3r, f3 := res["f3"]
	f4r, f4 := res["f4"]
	f5r, f5 := res["f5"]
	f6r, f6 := res["f6"]
	f7r, f7 := res["f7"]
	assert.Nil(t, err, "not error %s", err)
	assert.True(t, f1 && f2 && f1r.R == "f1 result" && f2r.R == "f2 result", "f1(%t)=%#v\n f2(%t)=%#v\n ", f1, f1r, f2, f2r)
	assert.True(t, f3 && f3r.R == "f2 result|f4 result|f3 result", "f3(%t)=%#v", f3, f3r)

	assert.True(t, f4 && f5 && f4r.E == nil && f4r.R == "f4 result" && f5r.E == nil && f5r.R == "f5 result", "f4(%t)=%#v\n f5(%t)=%#v", f4, f4r, f5, f5r)
	assert.True(t, f6 && f7 && f6r.E == nil && f6r.R == "f2 result|f4 result|f3 result|f6 result" && f7r.E == nil && f7r.R == "f7 result", "f6(%t)=%#v\n f7(%t)=%#v", f6, f6r, f7, f7r)

}

func TestDoWithErrorAtMiddle(t *testing.T) {
	symphony := New()

	symphony.Add("f2", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f2==")
		return "f2 result", nil
	})
	symphony.Add("f3", []string{"f2"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f3==")
		return "f3 result", nil
	})
	symphony.Add("f4", []string{"f5"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f4==")
		return "f4 result", nil
	})
	symphony.Add("f5", nil, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f5==")
		return "f5 result", errors.New("err5")
	})
	symphony.Add("f1", nil, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f1==")
		return "f1 result", nil
	})
	res, err := symphony.Do(context.Background(), 10000)

	f1r, f1 := res["f1"]
	f2r, f2 := res["f2"]
	f3r, f3 := res["f3"]
	f4r, f4 := res["f4"]
	f5r, f5 := res["f5"]
	assert.NotNil(t, err, "get error %s", err)
	assert.True(t, f1 && f2 && f3 && f1r.R == "f1 result" && f2r.R == "f2 result" && f3r.R == "f3 result", "  f1(%t)=%#v\n f2(%t)=%#v\n f3(%t)=%#v", f1, f1r, f2, f2r, f3, f3r)
	assert.True(t, f4 && f5 && f4r.E != nil && f4r.R == nil && f5r.E != nil && f5r.R == "f5 result", "f4(%t)=%#v\n f5(%t)=%#v", f4, f4r, f5, f5r)
}

func TestDoWithErrorAtLeaf(t *testing.T) {
	symphony := New()

	symphony.Add("f2", []string{"f1"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f2==")
		return "f2 result", nil
	})
	symphony.Add("f3", []string{"f2"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f3==")
		return "f3 result", nil
	})
	symphony.Add("f4", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f4==")
		return "f4 result", nil
	})
	symphony.Add("f5", []string{"f3"}, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f5==")
		return "f5 result", errors.New("err5")
	})
	symphony.Add("f1", nil, func(res map[string]*TaskState) (interface{}, error) {
		fmt.Println("==starting f1==")
		return "f1 result", nil
	})
	res, err := symphony.Do(context.Background(), 10000)

	f1r, f1 := res["f1"]
	f2r, f2 := res["f2"]
	f3r, f3 := res["f3"]
	f4r, f4 := res["f4"]
	f5r, f5 := res["f5"]
	assert.NotNil(t, err, "get error %s", err)
	assert.Equal(t, "err5", err.Error())
	assert.True(t, f1 && f2 && f3 && f1r.R == "f1 result" && f2r.R == "f2 result" && f3r.R == "f3 result", "  f1(%t)=%#v\n f2(%t)=%#v\n f3(%t)=%#v", f1, f1r, f2, f2r, f3, f3r)
	assert.True(t, f4 && f5 && f4r.E == nil && f4r.R == "f4 result" && f5r.E != nil && f5r.R == "f5 result", "f4(%t)=%#v\n f5(%t)=%#v", f4, f4r, f5, f5r)
}
