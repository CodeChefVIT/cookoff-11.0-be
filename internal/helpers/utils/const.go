package utils


type Judge0StatusID int

const (
	Judge0InQueue Judge0StatusID = 1
	Judge0Processing Judge0StatusID = 2
	Judge0Accepted Judge0StatusID = 3
	Judge0WrongAnswer Judge0StatusID = 4
	Judge0TimeLimitExceeded Judge0StatusID = 5
	Judge0CompilationError Judge0StatusID = 6
	Judge0RuntimeSIGSEGV Judge0StatusID = 7
	Judge0RuntimeSIGXFSZ Judge0StatusID = 8
	Judge0RuntimeSIGFPE Judge0StatusID = 9
	Judge0RuntimeSIGABRT Judge0StatusID = 10
	Judge0RuntimeNZEC Judge0StatusID = 11
	Judge0RuntimeOther Judge0StatusID = 12
	Judge0InternalError Judge0StatusID = 13
	Judge0ExecFormatError Judge0StatusID = 14
)


var Judge0StatusMap = map[Judge0StatusID]string{
	Judge0InQueue:  "In Queue",
	Judge0Processing:  "Processing",
	Judge0Accepted:  "Success",
	Judge0WrongAnswer:  "Wrong Answer",
	Judge0TimeLimitExceeded:  "Time Limit Exceeded",
	Judge0CompilationError:  "Compilation Error",
	Judge0RuntimeSIGSEGV:  "Runtime Error (SIGSEGV)",
	Judge0RuntimeSIGXFSZ:  "Runtime Error (SIGXFSZ)",
	Judge0RuntimeSIGFPE:  "Runtime Error (SIGFPE)",
	Judge0RuntimeSIGABRT: "Runtime Error (SIGABRT)",
	Judge0RuntimeNZEC: "Runtime Error (NZEC)",
	Judge0RuntimeOther: "Runtime Error (Other)",
	Judge0InternalError: "Internal Error",
	Judge0ExecFormatError: "Exec Format Error",
}

func (s Judge0StatusID)GetJudge0Status() string{
	if s, ok := Judge0StatusMap[s]; ok {
		return s
	}
	return "Invalid Status"
}



func GetJudge0StatusFromID(id int) string{
	if s, ok := Judge0StatusMap[Judge0StatusID(id)]; ok {
		return s
	}
	return "Invalid Status"
}

