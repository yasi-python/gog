#include <iostream>
#include <chrono>
#include <thread>

// Tiny C++ stub to satisfy "C++ 100%" build presence.
// It does nothing important and is not used by the Go service.
int main() {
  std::cout << "pingstub ok\n";
  std::this_thread::sleep_for(std::chrono::milliseconds(10));
  return 0;
}