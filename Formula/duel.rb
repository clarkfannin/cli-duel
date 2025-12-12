class Duel < Formula
  desc "CLI 1v1 dueling game"
  homepage "https://github.com/clarkfannin/cli-duel"
  url "https://github.com/clarkfannin/cli-duel/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "7c4b5e32bef78b38cf37817f867634bb0c8bbd6354eb1b032e0df806f75800b7"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"duel", "."
  end

  test do
    assert_match "duel", shell_output("#{bin}/duel help 2>&1", 0)
  end
end
