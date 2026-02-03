class CaddyLlmProxy < Formula
  desc "Caddy with LLM-based dynamic routing plugin"
  homepage "https://github.com/contember/ai-proxy"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-arm64.tar.gz"
      sha256 "ca4b749ec3d10b18373120c5f25282a37d11e2daa5a6d57d80b2fef0c48e80cf"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-amd64.tar.gz"
      sha256 "3eb4b3108c321211f800509e3b1b000576e7f0e45e0bc7ba9c07ef2a3737883a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-arm64.tar.gz"
      sha256 "79553b39516c253277a9fece6515595d90f6f59d4f2b78e597faacb42f8e87bf"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-amd64.tar.gz"
      sha256 "6b7411039591004701a8a12bf32e9b064eb005d55320f209f729cbe9174cfd60"
    end
  end

  def install
    bin.install "caddy" => "caddy-llm-proxy"
  end

  service do
    run [opt_bin/"caddy-llm-proxy", "run", "--config", etc/"Caddyfile"]
    keep_alive true
    log_path var/"log/caddy-llm-proxy.log"
    error_log_path var/"log/caddy-llm-proxy.error.log"
  end

  def caveats
    <<~EOS
      To use the LLM proxy, set the following environment variables:
        export LLM_API_KEY="your-api-key"

      Create a Caddyfile at #{etc}/Caddyfile with your configuration.

      To start the service:
        brew services start caddy-llm-proxy
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/caddy-llm-proxy version")
  end
end
