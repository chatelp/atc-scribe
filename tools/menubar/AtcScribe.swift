// atc-scribe menu bar launcher.
//
// An icon in the menu bar that says whether production runs, starts and stops
// it, and opens the page. It does not run production itself: it runs the same
// script a double-click runs (production_command, runs/production.command on the
// owner's station), so the sign-in, the station link and the clean stop stay in
// one place. Stopping sends that script SIGTERM, which its trap turns into a
// clean stop of co-atc, its sidecar and the station link. Nothing starts on its
// own: production runs only when asked (owner's choice, 10/10).
//
// Settings, all optional, in ~/.config/atc-scribe/launcher.json:
//   repo                the atc-scribe checkout (default: where it was built)
//   production_command  relative to repo (default: runs/production.command)
//   station_status_url  radio-ctl's /radio/etat, for the line "On air"
//   tailscale_url       copied by "Copy remote address"
//   planning_file       a markdown table of who uses the GPU; production adds
//                       its row while it runs and removes it when it stops

import AppKit
import SwiftUI

struct LauncherConfig: Decodable {
    var repo: String?
    var production_command: String?
    var station_status_url: String?
    var tailscale_url: String?
    var planning_file: String?
}

/// Replaced by build.sh with the checkout the app was built from.
let builtRepo = "@REPO@"

let coAtcPattern = "bin/co-atc -config configs/config.toml"
let testPattern = "runs/son-test/co-atc -config"
let planningMarker = "(menu bar launcher)"

@discardableResult
func run(_ path: String, _ args: [String]) -> (status: Int32, out: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: path)
    p.arguments = args
    let pipe = Pipe()
    p.standardOutput = pipe
    p.standardError = Pipe()
    do { try p.run() } catch { return (-1, "") }
    let data = pipe.fileHandleForReading.readDataToEndOfFile()
    p.waitUntilExit()
    return (p.terminationStatus, String(decoding: data, as: UTF8.self))
}

func pids(matching pattern: String) -> [Int32] {
    run("/usr/bin/pgrep", ["-f", pattern]).out
        .split(separator: "\n").compactMap { Int32($0.trimmingCharacters(in: .whitespaces)) }
}

@MainActor
final class Production: ObservableObject {
    @Published var running = false
    @Published var elapsed = ""
    @Published var onAir = "—"
    @Published var sidecar = "—"
    @Published var testInstance = false
    @Published var busy = false
    @Published var note = ""

    let cfg: LauncherConfig
    let repo: String
    var command: String { cfg.production_command ?? "runs/production.command" }
    private var timer: Timer?

    init() {
        let path = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/atc-scribe/launcher.json")
        if let data = try? Data(contentsOf: path),
           let c = try? JSONDecoder().decode(LauncherConfig.self, from: data) {
            cfg = c
        } else {
            cfg = LauncherConfig()
        }
        repo = cfg.repo ?? builtRepo
        timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in
            Task { @MainActor in await self?.refresh() }
        }
        Task { await refresh() }
    }

    func refresh() async {
        let co = pids(matching: coAtcPattern)
        running = !co.isEmpty
        testInstance = !pids(matching: testPattern).isEmpty
        if let pid = co.first {
            elapsed = run("/bin/ps", ["-o", "etime=", "-p", String(pid)]).out
                .trimmingCharacters(in: .whitespacesAndNewlines)
            sidecar = await sidecarState()
        } else {
            elapsed = ""
            sidecar = "—"
        }
        onAir = await stationOnAir()
        // A start ends when co-atc runs, a stop when the script is gone.
        if busy && (running ? note == "Starting…" : pids(matching: command).isEmpty) {
            busy = false
            note = ""
        }
    }

    func start() {
        guard !running, !busy else { return }
        let script = (repo as NSString).appendingPathComponent(command)
        guard FileManager.default.isExecutableFile(atPath: script) else {
            note = "Not found: \(script)"
            return
        }
        let log = (repo as NSString).appendingPathComponent("runs/launcher.log")
        FileManager.default.createFile(atPath: log, contents: nil)
        guard let out = FileHandle(forWritingAtPath: log) else { return }
        out.seekToEndOfFile()
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/sh")
        p.arguments = [script]
        p.currentDirectoryURL = URL(fileURLWithPath: repo)
        p.standardOutput = out
        p.standardError = out
        p.standardInput = FileHandle.nullDevice // no terminal: the password comes from its file
        do {
            try p.run()
            busy = true
            note = "Starting…"
            setPlanningRow(true)
        } catch {
            note = "Could not start: \(error.localizedDescription)"
        }
    }

    func stop() {
        // The script's trap stops co-atc, its sidecar and the station link. A
        // script started by a double-click in Terminal is stopped the same way.
        let scripts = pids(matching: command)
        if scripts.isEmpty {
            note = "No production script to stop"
            return
        }
        for pid in scripts { kill(pid, SIGTERM) }
        busy = true
        note = "Stopping…"
        setPlanningRow(false)
    }

    func open(_ url: String) {
        if let u = URL(string: url) { NSWorkspace.shared.open(u) }
    }

    func openLog() {
        NSWorkspace.shared.open(URL(fileURLWithPath: (repo as NSString).appendingPathComponent("runs/production.log")))
    }

    func copyRemoteAddress() {
        guard let url = cfg.tailscale_url else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(url, forType: .string)
        note = "Copied \(url)"
    }

    private func sidecarState() async -> String {
        guard let url = URL(string: "http://127.0.0.1:8178/health"),
              let (data, _) = try? await URLSession.shared.data(from: url),
              let j = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return "not answering" }
        let status = j["status"] as? String ?? "?"
        let degraded = (j["degraded"] as? [String]) ?? []
        return degraded.isEmpty ? status : "\(status): \(degraded.joined(separator: ", "))"
    }

    private func stationOnAir() async -> String {
        guard let s = cfg.station_status_url, let url = URL(string: s) else { return "—" }
        var req = URLRequest(url: url)
        req.timeoutInterval = 4
        guard let (data, _) = try? await URLSession.shared.data(for: req),
              let j = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let sel = j["selection"] as? [String: Any],
              let title = sel["titre"] as? String else { return "station not answering" }
        return title
    }

    // The GPU is shared with the lab: production says so while it runs, in the
    // row a person or an agent would otherwise have written by hand.
    private func setPlanningRow(_ on: Bool) {
        guard let path = cfg.planning_file,
              var text = try? String(contentsOfFile: path, encoding: .utf8) else { return }
        var lines = text.components(separatedBy: "\n").filter { !$0.contains(planningMarker) }
        if on, let sep = lines.firstIndex(where: { $0.hasPrefix("|---") }) {
            let f = DateFormatter()
            f.locale = Locale(identifier: "fr_FR")
            f.dateFormat = "dd/MM 'à' HH:mm"
            lines.insert("| depuis le \(f.string(from: Date())), durée ouverte \(planningMarker) | atc-scribe | **co-atc en production** | oui, en continu (trois modèles, ~8 Go) |", at: sep + 1)
        }
        text = lines.joined(separator: "\n")
        try? text.write(toFile: path, atomically: true, encoding: .utf8)
    }
}

// The same actions without the menu, for a terminal or a Shortcut:
//   atc-scribe --status | --start | --stop
// They go through the very code the menu's buttons run.
@main
enum Entry {
    static func main() {
        let args = Array(CommandLine.arguments.dropFirst())
        if let cmd = args.first, ["--status", "--start", "--stop"].contains(cmd) {
            MainActor.assumeIsolated { cli(cmd) }
            return
        }
        AtcScribeLauncher.main()
    }

    @MainActor static func cli(_ cmd: String) {
        let p = Production()
        func settle(_ seconds: Double) { RunLoop.main.run(until: Date().addingTimeInterval(seconds)) }
        func report() {
            print(p.running ? "production running" + (p.elapsed.isEmpty ? "" : " for \(p.elapsed)") : "production stopped")
            if p.running { print("sidecar: \(p.sidecar)") }
            print("on air: \(p.onAir)")
            if !p.note.isEmpty { print("note: \(p.note)") }
        }
        settle(2)
        switch cmd {
        case "--start":
            p.start()
            for _ in 0..<120 where !(p.running && p.sidecar == "ok") { settle(2); Task { await p.refresh() } }
        case "--stop":
            p.stop()
            for _ in 0..<60 where p.running { settle(2); Task { await p.refresh() } }
        default:
            break
        }
        Task { await p.refresh() }
        settle(3)
        report()
        exit(p.note.hasPrefix("Not found") || p.note.hasPrefix("Could not") ? 1 : 0)
    }
}

struct AtcScribeLauncher: App {
    @StateObject private var p = Production()

    var body: some Scene {
        MenuBarExtra {
            Text(p.running ? "Production running" + (p.elapsed.isEmpty ? "" : " for \(p.elapsed)") : "Production stopped")
            if p.running { Text("Sidecar: \(p.sidecar)") }
            Text("On air: \(p.onAir)")
            if p.testInstance { Text("Test instance running") }
            if !p.note.isEmpty { Text(p.note) }
            Divider()
            Button("Start production") { p.start() }.disabled(p.running || p.busy)
            Button("Stop production") { p.stop() }.disabled(!p.running)
            Divider()
            Button("Open co-atc") { p.open("http://127.0.0.1:8000/") }.disabled(!p.running)
            if p.cfg.tailscale_url != nil { Button("Copy remote address") { p.copyRemoteAddress() } }
            Button("Production log") { p.openLog() }
            Divider()
            Button("Quit launcher (production keeps running)") { NSApplication.shared.terminate(nil) }
        } label: {
            Image(systemName: p.running ? "dot.radiowaves.left.and.right" : "antenna.radiowaves.left.and.right.slash")
        }
        .menuBarExtraStyle(.menu)
    }
}
