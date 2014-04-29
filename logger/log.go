package logger

// level is to specified the log to print
// level<=0 including Debug,Info,Warn,Error,Fatal
// level=1 including Info,Warn,Error,Fatal
// level=2 including Warn,Error,Fatal
// level=3 including Error,Fatal
// level>=4 including Fatal

var Level int

//Debug to print some debug on screen
func Debug(s string) {
	if Level <= 0 {
		println("[DEBUG]: " + s)
	}
}

func Info(s string) {
	if Level <= 1 {
		println("[INFO] : " + s)
	}
}

func Warn(s string) {
	if Level <= 2 {
		println("[WARN] : " + s)
	}
}

func Error(s string) {
	if Level <= 3 {
		println("[ERROR]: " + s)
	}
}

func Fatal(s string) {
	println("[FATAL]: " + s)
}
