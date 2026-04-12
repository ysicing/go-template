export type BuildVersionInfo = {
  version?: string
  git_commit?: string
  build_date?: string
}

export function getBuildVersionLabel(info: BuildVersionInfo): string {
  const shortCommit = info.git_commit?.slice(0, 7) ?? ""
  const parts = [info.version, shortCommit].filter(Boolean)
  if (parts.length > 0) {
    return parts.join(" · ")
  }
  return info.build_date ?? ""
}

export function getBuildVersionDetails(info: BuildVersionInfo): string[] {
  return [
    info.version ? `Version: ${info.version}` : "",
    info.git_commit ? `Commit: ${info.git_commit}` : "",
    info.build_date ? `Build: ${info.build_date}` : "",
  ].filter(Boolean)
}
