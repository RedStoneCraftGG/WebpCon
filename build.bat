@echo off
REM Install dependencies and build for Windows

echo Installing Go dependencies...
go mod tidy

echo Building webpcon.exe...
go build -o webpcon.exe main.go

echo.
echo Done! You can run webpcon.exe now.