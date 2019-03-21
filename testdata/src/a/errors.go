package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func returnNil() error { // want `func returnNil #0: \[nil\]`
	return nil
}

func returnErrorsNew() error { // want `func returnErrorsNew #0: \[errors\.New#0\("errors new"\)\]`
	return errors.New("errors new")
}

func notReturn() { // not a error function
	_ = returnNil()
	return
}

func ifError(a int) error { // want `func ifError #0: \[errors.New#0\("even"\) errors.New#0\("odd"\)\]`
	var err error
	if a%2 == 0 {
		err = errors.New("even")
	} else {
		err = errors.New("odd")
	}
	return err
}

func nilAndErrorsNew() (error, error) { // want `func nilAndErrorsNew #0: \[nil\] #1: \[errors.New#0\("1"\)\]`
	return nil, errors.New("1")
}

func returnLeftOrRight() error { // want `func returnLeftOrRight #0: \[nilAndErrorsNew#0\(\) nilAndErrorsNew#1\(\) nil\]`
	err1, err2 := nilAndErrorsNew()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func ignoreError() error { // want `func ignoreError #0: \[nil\]`
	_, _ = nilAndErrorsNew()
	return nil
}

func wrap(err error, msg string) error { // want `func wrap #0: \[fmt\.Errorf#0\("%s: %v", \w+\)\]`
	return fmt.Errorf("%s: %v", msg, err)
}

func wrapError() error { // want `func wrapError #0: \[wrap#0\(errors\.New#0\("wrapping"\), "wrapped"\) nil\]`
	err := errors.New("wrapping")
	if err != nil {
		return wrap(err, "wrapped")
	}
	return nil
}

func databaseError() error { // want `func databaseError #0: \[database/sql\.Open#1\("", ""\) \(\*database/sql\.DB\).Exec#1\(\w+, "aaaaaa", nil\)\]`
	db, err := sql.Open("", "")
	if err != nil {
		return err
	}
	_, err = db.Exec("aaaaaa")
	return err
}

func namedReturn() (err error) { // want `func namedReturn #0: \[errors\.New#0\("a"\)\]`
	err = errors.New("a")
	return
}

func namedReturn2(a int) (err error) { // want `func namedReturn2 #0: \[errors\.New#0\("a > 0"\) errors\.New#0\("a <= 0"\)\]`
	if a > 0 {
		return errors.New("a > 0")
	} else {
		err = errors.New("a <= 0")
		return
	}
}

func namedReturn3() (err error) { // want `func namedReturn3 #0: \[nil\]`
	return
}

func deferReturn() (err error) { // want `func deferReturn #0: \[modified by \[deferReturn\$1\]\]`
	defer func() {
		err = errors.New("func b")
	}()
	return
}

func modifiedByLambda() error { // want `func modifiedByLambda #0: \[nilAndErrorsNew#0\(\) modified by \[modifiedByLambda\$1\]\]`
	err1, err2 := nilAndErrorsNew()
	func() {
		err1 = err2
	}()
	return err1
}

func errorChannel() error { // want `func errorChannel #0: \[errors.New#0\("a"\)\]`
	ch := make(chan error)
	defer close(ch)
	ch <- errors.New("a")
	return <-ch
}

// channel: [errors.New("a"), errors.New("b")]
func errorChannel2() error { // want `func errorChannel2 #0: \[errors.New#0\("a"\) errors\.New#0\("b"\)\]`
	ch := make(chan error)
	defer close(ch)
	ch <- errors.New("a")
	<-ch
	ch <- errors.New("b")
	return <-ch
}

func gorutineAndChannel() error { // want `func gorutineAndChannel #0: \[modified by \[gorutineAndChannel\$1\]\]`
	ch := make(chan error)
	defer close(ch)
	for i := 0; i < 2; i++ {
		go func(id int) {
			ch <- fmt.Errorf("gorutine %d", id)
		}(i)
	}
	err := <-ch
	return err
}

func gorutineAndChannel2() (error, error) { // want `func gorutineAndChannel2 #0: \[errors\.New#0\("detect"\) modified by \[gorutineAndChannel2\$1\]\] #1: \[errors.New#0\("detect"\) modified by \[gorutineAndChannel2\$1\]\]`
	ch := make(chan error)
	defer close(ch)
	ch <- errors.New("detect")
	go func() {
		ch <- errors.New("gorutine")
	}()
	return <-ch, <-ch
}

var gerr error

func returnGlobal() error { // want `func returnGlobal #0: \[global\(gerr\)\]`
	return gerr
}

func returnAnonymous() error { // want `func returnAnonymous #0: \[returnAnonymous\$1#0\(\)\]`
	return func() error { // want `func returnAnonymous\$1 #0: \[errors.New#0\("anonymous"\)\]`
		return errors.New("anonymous")
	}()
}

func sliceError() error { // want `func sliceError #0: \[memory access\]`
	arr := make([]error, 0)
	arr = append(arr, returnNil())
	arr = append(arr, returnNil())
	return arr[0]
}

func arrayError() error { // want `func arrayError #0: \[memory access\]`
	var errs [2]error
	errs[0] = returnNil()
	errs[1] = returnNil()
	return errs[1]
}

func mapError() error { // want `func mapError #0: \[memory access\]`
	errMap := make(map[int]error, 0)
	errMap[5] = errors.New("5")
	errMap[3] = errors.New("3")
	return errMap[3]
}

/* cannot detect
func phi() (err error) { // want `func phi #0: \[returnNil#0\(\) returnGlobal\(\) nil\]`
	for i := 0; i < 10; i++ {
		if i < 10 {
			err = returnNil()
			if err != nil {
				return
			}
			err = returnGlobal()
			if err != nil {
				return
			}
		}
	}
	return
}

func typeAssert() error { // want `func typeAssert #0: \[errors.New#0\("interface"\)\]`
	var interf interface{}
	interf = errors.New("interface")
	return interf.(error)
}
*/
