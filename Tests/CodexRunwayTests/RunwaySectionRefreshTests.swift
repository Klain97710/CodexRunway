import Foundation
import Testing
@testable import CodexRunway
@testable import CodexRunwayCore

@Suite("Section refresh failures")
@MainActor
struct RunwaySectionRefreshTests {
    @Test("a first failure remains distinct from zero and does not hide another section",
          arguments: [true, false], [true, false])
    func firstFailure(quotaFails: Bool, full: Bool) async throws {
        let fixture = SectionRefreshFixture()
        defer { fixture.remove() }
        fixture.quotaFails = quotaFails
        fixture.creditsFail = !quotaFails
        try await fixture.refresh(full: full)

        #expect((fixture.model.quotaRefreshError != nil) == quotaFails)
        #expect((fixture.model.resetCreditsRefreshError != nil) == !quotaFails)
        #expect(fixture.model.quotaMeters.isEmpty == quotaFails)
        #expect((fixture.model.quotaUpdatedAt == nil) == quotaFails)
        #expect((fixture.model.resetCreditSummary == nil) == !quotaFails)
        if quotaFails { #expect(fixture.model.resetCreditSummary?.availableCount == 2) }
    }

    @Test("transient failures keep data and timestamps until a successful refresh", arguments: [true, false])
    func preservesDataAndRecovers(full: Bool) async throws {
        let fixture = SectionRefreshFixture()
        defer { fixture.remove() }
        try await fixture.refresh(full: full)
        let meters = fixture.model.quotaMeters.map(\.usedPercent)
        let fetchedAt = fixture.updatedAt
        fixture.quotaFails = true
        fixture.creditsFail = true
        fixture.updatedAt = fetchedAt.addingTimeInterval(60)

        try await fixture.refresh(full: full)
        fixture.model.relabel()

        #expect(fixture.model.quotaMeters.map(\.usedPercent) == meters)
        #expect(fixture.model.quotaUpdatedAt == fetchedAt)
        #expect(fixture.model.resetCreditSummary?.availableCount == 2)
        #expect(fixture.model.resetCreditSummary?.updatedAt == fetchedAt)
        #expect(fixture.model.quotaRefreshError == URLError(.timedOut).localizedDescription)
        #expect(fixture.model.resetCreditsRefreshError == URLError(.timedOut).localizedDescription)

        fixture.quotaFails = false
        fixture.creditsFail = false
        try await fixture.refresh(full: full)
        #expect(fixture.model.quotaRefreshError == nil)
        #expect(fixture.model.resetCreditsRefreshError == nil)
        #expect(fixture.model.quotaUpdatedAt == fixture.updatedAt)
        #expect(fixture.model.resetCreditSummary?.updatedAt == fixture.updatedAt)
    }

    @Test("switching accounts clears both stale data and section errors")
    func accountChangeClearsFailures() async throws {
        let fixture = SectionRefreshFixture()
        defer { fixture.remove() }
        try await fixture.refresh(full: false)
        fixture.quotaFails = true
        fixture.creditsFail = true
        try await fixture.refresh(full: false)
        fixture.auth = SectionRefreshFixture.auth("account-b")
        fixture.quotaFails = false
        try await fixture.refreshQuota()

        #expect(!fixture.model.quotaMeters.isEmpty)
        #expect(fixture.model.quotaRefreshError == nil)
        #expect(fixture.model.resetCreditSummary == nil)
        #expect(fixture.model.resetCreditsRefreshError == nil)
    }

    @Test("late errors from another account cannot populate the current card")
    func lateErrorIsDiscarded() async throws {
        let fixture = SectionRefreshFixture()
        defer { fixture.releaseCredits(); fixture.remove() }
        try await fixture.refresh(full: false)
        fixture.creditsFail = true
        fixture.holdCredits = true
        fixture.model.refreshResetCredits()
        try await fixture.wait { fixture.creditsAreHeld }
        fixture.auth = SectionRefreshFixture.auth("account-b")
        try await fixture.refreshQuota()
        fixture.releaseCredits()
        try await fixture.wait { !fixture.model.isRefreshing(.resetCredits) }

        #expect(!fixture.model.quotaMeters.isEmpty)
        #expect(fixture.model.quotaRefreshError == nil)
        #expect(fixture.model.resetCreditSummary == nil)
        #expect(fixture.model.resetCreditsRefreshError == nil)
    }

    @Test("authentication failure clears data instead of presenting it as stale")
    func authenticationFailureClearsData() async throws {
        let fixture = SectionRefreshFixture()
        defer { fixture.remove() }
        try await fixture.refresh(full: false)
        fixture.quotaFails = true
        fixture.errorCode = .userAuthenticationRequired
        try await fixture.refreshQuota()

        #expect(fixture.model.quotaMeters.isEmpty)
        #expect(fixture.model.quotaUpdatedAt == nil)
        #expect(fixture.model.resetCreditSummary == nil)
        #expect(fixture.model.quotaRefreshError != nil)
        #expect(fixture.model.resetCreditsRefreshError != nil)
    }
}

@MainActor
private final class SectionRefreshFixture {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("runway-section-refresh-\(UUID().uuidString)")
    let suiteName = "runway-section-refresh-\(UUID().uuidString)"
    var auth = SectionRefreshFixture.auth("account-a")
    var updatedAt = Date(timeIntervalSince1970: 1_790_880_000)
    var quotaFails = false
    var creditsFail = false
    var errorCode = URLError.Code.timedOut
    var holdCredits = false
    private var creditsContinuation: CheckedContinuation<Void, Never>?
    private var quotaCalls = 0
    private var creditsCalls = 0
    var creditsAreHeld: Bool { creditsContinuation != nil }
    lazy var model: RunwayModel = makeModel()

    private func makeModel() -> RunwayModel {
        let settings = RunwaySettings(store: PreferencesStore(defaults: UserDefaults(suiteName: suiteName)!))
        settings.updateShowsQuotaEstimateSummary(false)
        settings.updateShowsCostSummary(false)
        settings.updateShowsTokenUsageHeatmap(false)
        settings.updateShowsSessionRepairSummary(false)
        settings.updateShowsRecentSessions(false)
        settings.updateShowsRateLimitResetToday(false)
        settings.updateQuotaAlertsEnabled(false)
        settings.updateResetCreditAlertsEnabled(false)
        settings.updateExportsStatusJSON(false)
        let services = RunwayModelServices(
            loadValidAuth: { _, _ in await self.auth },
            fetchQuota: { _ in try await self.quota() },
            fetchResetCredits: { _ in try await self.credits() },
            fetchRateLimitResetToday: { throw URLError(.unsupportedURL) },
            scanAPIEquivalent: { _, _, _, _ in throw URLError(.unsupportedURL) },
            fetchDailyWorkspaceUsage: { _, _, _, _, _ in throw URLError(.unsupportedURL) },
            fetchCodexProfileTokenUsage: { _ in throw URLError(.unsupportedURL) },
            dryRunSessions: { throw URLError(.unsupportedURL) },
            scanRecentSessions: { _ in throw URLError(.unsupportedURL) })
        return RunwayModel(
            settings: settings, services: services,
            accountStore: AccountStore(rootURL: root.appendingPathComponent("accounts"), officialAuthURL: root.appendingPathComponent("auth.json")),
            costCacheStore: UsageCostCacheStore(cacheURL: root.appendingPathComponent("cost.json")),
            quotaEstimateHistoryStore: QuotaEstimateHistoryStore(fileURL: root.appendingPathComponent("history.json")),
            grokCLIAvailable: false)
    }

    private func quota() throws -> QuotaSnapshot {
        quotaCalls += 1
        if quotaFails { throw URLError(errorCode) }
        return QuotaSnapshot(plan: "plus",
            primary: RateWindow(usedPercent: 25, windowMinutes: 300, resetsAt: updatedAt.addingTimeInterval(3600)),
            secondary: nil, additionalWindows: [], creditsBalance: nil, updatedAt: updatedAt)
    }

    private func credits() async throws -> ResetCreditsSnapshot {
        creditsCalls += 1
        let fails = creditsFail
        if holdCredits { await withCheckedContinuation { creditsContinuation = $0 } }
        if fails { throw URLError(errorCode) }
        return ResetCreditsSnapshot(availableCount: 2, credits: [], updatedAt: updatedAt)
    }

    func refresh(full: Bool) async throws {
        if full {
            model.refresh()
            try await wait { !model.isRefreshingAll }
        } else {
            try await refreshQuota()
            let count = creditsCalls
            model.refreshResetCredits()
            try await wait { creditsCalls > count && !model.isRefreshing(.resetCredits) }
        }
    }

    func refreshQuota() async throws {
        let count = quotaCalls
        model.refreshQuota()
        try await wait { quotaCalls > count && !model.isRefreshing(.quota) }
    }

    func wait(_ predicate: () -> Bool) async throws {
        for _ in 0..<200 {
            if predicate() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        try #require(predicate(), "Timed out waiting for section refresh")
    }

    func releaseCredits() {
        holdCredits = false
        creditsContinuation?.resume()
        creditsContinuation = nil
    }

    func remove() {
        try? FileManager.default.removeItem(at: root)
        UserDefaults.standard.removePersistentDomain(forName: suiteName)
    }

    static func auth(_ id: String) -> CodexAuth {
        CodexAuth(authMode: "chatgpt", tokens: .init(accessToken: "fixture-access-\(id)", refreshToken: "", accountId: id), lastRefresh: nil)
    }
}
