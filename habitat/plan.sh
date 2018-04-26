pkg_name=circonus-agent
pkg_origin=bixu
<<<<<<< HEAD
pkg_maintainer="Blake Irvin <blake.irvin@gmail.com>"
pkg_license=("BSD-3")
pkg_deps=(
  core/cacerts
  core/coreutils
=======
pkg_version=0.13.0
pkg_maintainer="Blake Irvin <blake.irvin@gmail.com>"
pkg_license=("BSD-3")
pkg_source="https://github.com/circonus-labs/${pkg_name}/releases/download/v${pkg_version}/${pkg_name}_${pkg_version}_linux_64-bit.tar.gz"
pkg_shasum="e04eb36dff44f6c6e103337d158fef135f887536cca7c79fe9d3003e25b5159b"
pkg_deps=(
  core/cacerts
  core/coreutils
  core/go
>>>>>>> WIP
  core/grep
  core/runit
  core/sed
)
<<<<<<< HEAD
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
=======
pkg_build_deps=()
pkg_bin_dirs=(bin)
pkg_svc_user="hab"

do_setup_environment() {
  set_runtime_env SSL_CERT_DIR $(pkg_path_for core/cacerts)/ssl/certs/
>>>>>>> WIP
  return $?
}

do_build() {
<<<<<<< HEAD
  pushd "${pkg_cache_path}" >/dev/null
    GOOS=linux go build -o "${GOPATH}/bin/${pkg_name}" ./
  popd >/dev/null
  return $?
}

do_install() {
  cp -r "${GOPATH}/bin"                "${pkg_prefix}/"
  cp -r "$PLAN_CONTEXT/../plugins"     "${pkg_prefix}/plugins"
  mv "${pkg_prefix}/plugins/README.md" "${pkg_prefix}/README.md"
=======
  return 0
}

do_install() {
  cp -pr $HAB_CACHE_SRC_PATH/sbin/*  $pkg_prefix/bin
  cp -pr $HAB_CACHE_SRC_PATH/etc     $pkg_prefix/etc
  mkdir                              $pkg_prefix/plugins
  cp -pr /src/plugins/*              $pkg_prefix/plugins
  chmod  +x                          $pkg_prefix/plugins/*
>>>>>>> WIP
  return $?
}
