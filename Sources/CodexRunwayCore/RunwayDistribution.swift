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

    /// Shared by direct Sparkle downloads and the custom-proxy bridge.
    public static func isAllowedUpdateURL(_ url: URL) -> Bool {
        guard url.scheme?.lowercased() == "https", url.host?.lowercased() == "github.com",
              url.user == nil, url.password == nil, url.fragment == nil,
              url.port == nil || url.port == 443
        else { return false }
        let parts = url.path.split(separator: "/", omittingEmptySubsequences: false)
        guard parts.count == 7, parts[0].isEmpty,
              "\(parts[1])/\(parts[2])".lowercased() == repository.lowercased(),
              parts[3] == "releases", !parts[5].isEmpty, !parts[6].isEmpty,
              !parts.contains("."), !parts.contains(".."), !url.path.contains("\\")
        else { return false }
        return parts[4] == "download" || (parts[4] == "latest" && parts[5] == "download")
    }
}
