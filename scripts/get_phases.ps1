param([string]$hex = "c0884e", [int]$port = 8000)
$r = Invoke-RestMethod "http://127.0.0.1:${port}/api/v1/aircraft/${hex}/tracks?limit=1000"

Write-Output "=== Phase History ==="
$r.phase_history | Format-Table phase, timestamp -AutoSize

Write-Output "`n=== Track data (first 60 + last 20) ==="
Write-Output "Timestamp            | Alt    | GS     | VS    | Dist  | Hdg    | OnGnd"
Write-Output "---------------------+--------+--------+-------+-------+--------+------"
$h = $r.history
$total = $h.Count
$show = @()
if ($total -le 80) { $show = $h }
else { $show = $h[($total-60)..($total-1)] + $h[0..19] }
foreach ($p in $show) {
    "{0} | {1,6} | {2,6:F1} | {3,5} | {4,5} | {5,6:F1} | {6}" -f $p.timestamp, $p.altitude, $p.speed_gs, $p.vertical_speed, $p.distance, $p.true_heading, $p.on_ground
}
