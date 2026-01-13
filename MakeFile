EXE = gooseengine

ifeq ($(OS),Windows_NT)
    EXE = gooseengine.exe
endif

all:
	go build -o $(EXE)

clean:
	rm -f $(EXE) gooseengine gooseengine.exe
