package cogs

// import (
//     "fmt"
//     "strings"
// )

// Errors raised by package x.
const (
	ErrNoEncAndNoDecrypt = errConst("NoEnc and NoDecrypt cannot both be true")
)

type errConst string

func (err errConst) Error() string {
	return string(err)
}

// func (err errConst) Is(target error) bool {
//     ts := target.Error()
//     es := string(err)
//     return ts == es || strings.HasPrefix(ts, es+": ")
// }

// func (err errConst) wrap(inner error) error {
//     return errWrap{msg: string(err), err: inner}
// }

// type errWrap struct {
//     err error
//     msg string
// }

// func (err errWrap) Error() string {
//     if err.err != nil {
//         return fmt.Sprintf("%s: %v", err.msg, err.err)
//     }
//     return err.msg
// }
// func (err errWrap) Unwrap() error {
//     return err.err
// }
// func (err errWrap) Is(target error) bool {
//     return errConst(err.msg).Is(target)
// }
