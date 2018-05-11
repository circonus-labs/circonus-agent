pkg_name=circonus-agent
pkg_origin=bixu
pkg_maintainer="Blake Irvin <blake.irvin@gmail.com>"
pkg_license=("BSD-3")
pkg_deps=(
  core/cacerts
  core/coreutils
  core/grep
  core/runit
  core/sed
)
pkg_build_deps=(
  core/go
  core/git
  core/dep
)
pkg_bin_dirs=(bin)
pkg_svc_user="root"

pkg_version() {
  git tag --sort="version:refname" | tail --lines=1 | cut --delimiter=v --fields=2
}

do_setup_environment() {
  set_runtime_env   SSL_CERT_DIR   $(pkg_path_for core/cacerts)/ssl/certs/
  export GOPATH="${HAB_CACHE_SRC_PATH}/go"
  export workspace_src="${GOPATH}/src"
  export base_path="github.com/circonus-labs"
  export pkg_cache_path="${workspace_src}/${base_path}/${pkg_name}"
  return $?
}

do_before() {
  update_pkg_version
  return $?
}

do_download() {
  return 0
}

do_prepare() {
  mkdir -p "$pkg_cache_path"
  cp -r "${PLAN_CONTEXT}/../"* "$pkg_cache_path"
  pushd "${pkg_cache_path}" >/dev/null
    dep ensure
  popd >/dev/null
  return $?
}

do_build() {
  pushd "${pkg_cache_path}" >/dev/null
    GOOS=linux go build -o "${GOPATH}/bin/${pkg_name}" ./
  popd >/dev/null
  return $?
}

do_install() {
  cp -r "${GOPATH}/bin"              "${pkg_prefix}/${bin}"
  return $?
}
