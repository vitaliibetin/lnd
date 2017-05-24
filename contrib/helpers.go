package contrib

//import (
//	"os"
//	"fmt"
//	"path/filepath"
//	"time"
//)
//
//func PrintlnToFile(fileName string, args ...interface{}) {
//	dirName := filepath.Dir(fileName)
//	err := os.MkdirAll(dirName, 0755)
//	if err != nil {
//		panic("Error creating required libraries:" + err.Error())
//	}
//	f, err := os.OpenFile(fileName, os.O_APPEND | os. O_WRONLY | os.O_CREATE, 0644)
//	if err != nil {
//		panic("Error opening file: "+err.Error())
//	}
//	margs := []interface{} {
//		time.Now().Format("15:04:05.999"),
//		fmt.Sprintf("[%v]", os.Getpid()),
//	}
//	margs = append(margs, args...)
//	_, err = f.WriteString(fmt.Sprintln(margs...))
//	if err != nil {
//		panic("Error writing to file: "+err.Error())
//	}
//	err = f.Close()
//	if err != nil {
//		panic("Error closing file: "+err.Error())
//	}
//}
