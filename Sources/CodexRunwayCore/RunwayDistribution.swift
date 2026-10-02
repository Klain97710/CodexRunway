import Foundation

/// Release identity is independent of the upstream source-code remote.
public enum RunwayDistribution {
    public static let repository = "Klain97710/CodexRunway"
    public static let maintainer = "Klain97710"
    public static let repositoryURL = URL(string: "https://github.com/\(repository)")!
    public static let issuesURL = repositoryURL.appendingPathComponent("issues/new")
    public static let upstreamURL = URL(string: "https://github.com/Licoy/CodexRunway")!
    public static let updatesEnabledInfoKey = "RunwayUpdatesEnabled"

    public static func appcastURL(architecture: String) -> URL {
        repositoryURL.appendingPathComponent("releases/latest/download/appcast-\(architecture).xml")
    }
}
