import kcl_plugin.os

{
  "os.exec": os.exec("echo", ["Hello, World!"])
  "os.exec(env)": os.exec("bash", ["-c", 'echo "Hello, $LOCATION!"'], env={"LOCATION": "World"})
}
