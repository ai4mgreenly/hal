# R-K7DK-LSJ6: when the locally-launched service receives SIGINT or
# SIGTERM, the process exits within 1 second regardless of what
# requests are in flight — including the long-lived live-update
# channel (R-K65O-80SH) the spec calls out by name.
require "rails_helper"
require "socket"

RSpec.describe "R-K7DK-LSJ6 launched-service shutdown deadline" do
  PORT = 4577
  DEADLINE_SECONDS = 1.0
  BOOT_TIMEOUT = 30.0
  EXIT_POLL_INTERVAL = 0.02

  def spawn_launched_service
    log_path = Rails.root.join("tmp/shutdown_deadline_launch.log").to_s
    env = {
      # R-68WP-XVCK requires these in development; their values do not
      # influence the shutdown property under test, so dummies suffice.
      "GOOGLE_CLIENT_ID" => "smoke-dummy",
      "GOOGLE_CLIENT_SECRET" => "smoke-dummy",
      "PORT" => PORT.to_s
    }
    # Stale pid files from a crashed earlier run would make `rails s` refuse
    # to boot ("A server is already running"). Remove any leftover before
    # this attempt; nothing else writes here during the test.
    pid_file = Rails.root.join("tmp/pids/server.pid")
    File.delete(pid_file) if File.exist?(pid_file)
    pid = Process.spawn(
      env,
      Rails.root.join("launch.sh").to_s,
      pgroup: true,
      chdir: Rails.root.to_s,
      out: log_path,
      err: [ :child, :out ]
    )
    [ pid, Process.getpgid(pid), log_path ]
  end

  def wait_for_port_ready(port, timeout)
    deadline = Time.now + timeout
    loop do
      begin
        TCPSocket.new("127.0.0.1", port).close
        return true
      rescue Errno::ECONNREFUSED, Errno::EADDRNOTAVAIL
        return false if Time.now > deadline
        sleep 0.1
      end
    end
  end

  def open_counter_stream(port)
    sock = TCPSocket.new("127.0.0.1", port)
    sock.write(
      "GET /counter/stream HTTP/1.1\r\n" \
      "Host: localhost\r\n" \
      "Accept: text/event-stream\r\n" \
      "Connection: keep-alive\r\n\r\n"
    )
    # Read at least the response headers so we know the SSE handler is
    # actually running in the server process when we signal it.
    sock.readpartial(512)
    sock
  end

  def measure_exit(pid, pgid, signal)
    started = Time.now
    Process.kill(signal, -pgid)
    loop do
      reaped = Process.waitpid(pid, Process::WNOHANG) rescue pid
      return Time.now - started if reaped
      return Float::INFINITY if Time.now - started > 5.0
      sleep EXIT_POLL_INTERVAL
    end
  end

  def reap_process_group(pgid)
    Process.kill("KILL", -pgid)
  rescue Errno::ESRCH
    # Already gone.
  end

  %w[INT TERM].each do |signal|
    it "exits within #{DEADLINE_SECONDS}s of SIG#{signal} with /counter/stream open" do
      pid, pgid, log_path = spawn_launched_service
      begin
        unless wait_for_port_ready(PORT, BOOT_TIMEOUT)
          log = File.read(log_path) rescue "(no log)"
          raise "launch.sh failed to open port #{PORT} within #{BOOT_TIMEOUT}s; log:\n#{log}"
        end
        stream_sock = open_counter_stream(PORT)
        # Give the handler a beat to settle into its hold-open loop so
        # the signal arrives while the long-lived response is genuinely
        # in flight, not still negotiating headers.
        sleep 0.2
        elapsed = measure_exit(pid, pgid, signal)
        expect(elapsed).to be < DEADLINE_SECONDS,
                           "process took #{elapsed.inspect}s to exit after SIG#{signal} " \
                           "(deadline #{DEADLINE_SECONDS}s); server log tail:\n" \
                           "#{File.read(log_path).lines.last(15).join rescue '(no log)'}"
      ensure
        stream_sock&.close
        reap_process_group(pgid)
        Process.waitpid(pid, Process::WNOHANG) rescue nil
      end
    end
  end
end
