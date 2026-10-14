import Foundation
@testable import CodexRunwayCore

@main struct Probe {
    static func main() async throws {
        let args = CommandLine.arguments
        let mode = args[1]
        let dir = URL(fileURLWithPath: args[2], isDirectory: true)
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { d in
            let raw = try d.singleValueContainer().decode(String.self)
            let f = ISO8601DateFormatter()
            f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            if let date = f.date(from: raw) { return date }
            f.formatOptions = [.withInternetDateTime]
            guard let date = f.date(from: raw) else { throw NSError(domain: "fixture date", code: 1) }
            return date
        }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys, .prettyPrinted]
        encoder.dateEncodingStrategy = .custom { date, e in
            let f = ISO8601DateFormatter()
            f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            var c = e.singleValueContainer()
            try c.encode(f.string(from: date))
        }
        func read<T: Decodable>(_ type: T.Type, _ path: URL) throws -> T {
            try decoder.decode(type, from: Data(contentsOf: path))
        }
        func save<T: Encodable>(_ value: T, _ name: String) throws {
            let url = dir.appendingPathComponent(name)
            try encoder.encode(value).write(to: url)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
        }
        if mode == "emit" {
            let repo = URL(fileURLWithPath: args[3], isDirectory: true)
            try save(read(RunwayPreferences.self, repo.appendingPathComponent("internal/store/testdata/swift_all.json")), "swift-preferences.json")
            try save(read(AccountIndex.self, repo.appendingPathComponent("internal/codex/testdata/swift_index.json")), "swift-index.json")
            try await verifyUsage(dir, "swift.sqlite3", reuse: false)
        } else if mode == "verify" {
            var expected = try read(RunwayPreferences.self, dir.appendingPathComponent("swift-preferences.json"))
            expected.selectedProvider = .codex
            expected.statusBarProviderScope = .selected
            let actual = try read(RunwayPreferences.self, dir.appendingPathComponent("go-preferences.json"))
            guard actual == expected else { throw NSError(domain: "Swift preference readback changed unrelated fields", code: 2) }
            let original = try read(AccountIndex.self, dir.appendingPathComponent("swift-index.json"))
            let result = try read(AccountIndex.self, dir.appendingPathComponent("go-index.json"))
            guard original == result else { throw NSError(domain: "Swift account/date/cache readback mismatch", code: 3) }
            try await verifyUsage(dir, "swift.sqlite3", reuse: true)
            try await verifyUsage(dir, "go.sqlite3", reuse: true)
            print("PASS: pinned Swift → Go → Swift; preferences, account IDs/order, metadata, cached quota and dates preserved")
        } else { throw NSError(domain: "unknown probe mode", code: 4) }
    }
    static func verifyUsage(_ dir: URL, _ name: String, reuse: Bool) async throws {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "Asia/Shanghai")!
        let repo = UsageCostRepository(codexHome: dir.appendingPathComponent("home/.codex"), databaseURL: dir.appendingPathComponent(name), parserVersion: 4, priceBook: .current, calendar: calendar)
        let f = ISO8601DateFormatter()
        let query = ApiCostQuery(id: "full", window: DateInterval(start: f.date(from: "2026-06-27T00:00:00Z")!, end: f.date(from: "2026-06-30T00:00:00Z")!))
        let result = try await repo.summaries(for: [query], calculatedAt: f.date(from: "2026-06-30T12:00:00Z")!, policy: .ifChanged)
        guard let summary = result["full"], summary.totals.totalTokens == 4000,
              summary.estimatedUSD == Decimal(string: "0.021") else { throw NSError(domain: "Swift SQLite totals/cost mismatch", code: 5) }
        let diagnostic = await repo.diagnosticsSnapshot()
        if reuse && (diagnostic.databaseRebuilds != 0 || diagnostic.rebuiltFiles != 0 || diagnostic.bytesRead > 105 || diagnostic.cacheHits == 0) {
            throw NSError(domain: "Swift rebuilt Go-written SQLite state", code: 6)
        }
        print("PASS: SQLite \(name), reuse=\(reuse), timezone Asia/Shanghai, 4000 tokens, exact decimal cost")
    }

}
