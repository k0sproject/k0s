Param([String] $Namespace)

$ErrorActionPreference = "Stop"
$stderr = [System.Console]::Error

$pong = [System.IO.Pipes.NamedPipeServerStream]::new((Join-Path -Path $Namespace -ChildPath 'pong'))
try {
    $stderr.WriteLine("${PID}: Sending ping")
    $ping = [System.IO.Pipes.NamedPipeClientStream]::new((Join-Path -Path $Namespace -ChildPath 'ping'))
    try {
        $ping.Connect()
        $writer = [System.IO.StreamWriter]::new($ping)
        $writer.WriteLine("ping from ${PID}")
        $writer.Flush()
        $ping.WaitForPipeDrain()
    }
    finally {
        $ping.Close()
        $ping.Dispose()
    }

    $stderr.WriteLine("${PID}: Awaiting pong")
    $pong.WaitForConnection()
    [System.IO.StreamReader]::new($pong).ReadToEnd() | Out-Null
}
finally {
    $pong.Close()
    $pong.Dispose()
}

$stderr.WriteLine("${PID}: Goodbye")
