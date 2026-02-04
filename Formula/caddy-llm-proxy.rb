class CaddyLlmProxy < Formula
  desc "Caddy with LLM-based dynamic routing plugin"
  homepage "https://github.com/contember/ai-proxy"
  version "0.1.1"
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
    bin.install "caddy" => "caddy-llm-proxy-bin"
    (etc/"caddy-llm-proxy").mkpath
    (etc/"caddy-llm-proxy").install "Caddyfile" unless (etc/"caddy-llm-proxy/Caddyfile").exist?

    # Create wrapper script that loads env file
    (bin/"caddy-llm-proxy").write <<~EOS
      #!/bin/bash
      set -a
      [ -f "#{etc}/caddy-llm-proxy/env" ] && source "#{etc}/caddy-llm-proxy/env"
      set +a
      export CADDY_DATA_DIR="${CADDY_DATA_DIR:-#{var}/lib/caddy-llm-proxy}"
      exec "#{opt_bin}/caddy-llm-proxy-bin" "$@"
    EOS
    (bin/"caddy-llm-proxy").chmod 0755
  end

  service do
    run [opt_bin/"caddy-llm-proxy", "run", "--config", etc/"caddy-llm-proxy/Caddyfile"]
    keep_alive true
    log_path var/"log/caddy-llm-proxy.log"
    error_log_path var/"log/caddy-llm-proxy.error.log"
  end

  def post_install
    (var/"lib/caddy-llm-proxy").mkpath
    # Create example env file if it doesn't exist
    env_file = etc/"caddy-llm-proxy/env"
    unless env_file.exist?
      env_file.write <<~EOS
        # Add your API key here:
        # LLM_API_KEY=sk-your-key-here
        #
        # Optional settings:
        # LLM_API_URL=https://openrouter.ai/api/v1/chat/completions
        # MODEL=anthropic/claude-haiku-4.5
      EOS
    end
  end

  def caveats
    <<~EOS
      Add your API key to #{etc}/caddy-llm-proxy/env:
        echo "LLM_API_KEY=sk-your-key" >> #{etc}/caddy-llm-proxy/env

      Start the service:
        sudo brew services start caddy-llm-proxy

      Logs: #{var}/log/caddy-llm-proxy.log
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/caddy-llm-proxy version")
  end
end
