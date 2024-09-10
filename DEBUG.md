# Guide to debug Inventory-api using vscode

## Prerequsites

* Install: <https://code.visualstudio.com/docs/languages/go>
* Install: https://github.com/go-delve/delve

Refer [vscode-go debugging guide](https://github.com/golang/vscode-go/blob/master/docs/debugging.md)

## Configuration

You can navigate to `Run` and click on `Add Configuration`. Select the installed `Go Launch` option. 
Or 
you can create a `launch.json` file in the directory `.vscode`. And copy the launch.json configuration

```
{
    // Use IntelliSense to learn about possible attributes.
    // Hover to view descriptions of existing attributes.
    // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${fileDirname}",
            "args":["serve","--config", ".inventory-api.yaml"]
        }
    ]
}
```
## Debugging

* Set the Break point on the code line.

* Navigate to the `Run` from Vscode menu and click on `Start Debugging`.

* You can see the output in the `DEBUG CONSOLE`



