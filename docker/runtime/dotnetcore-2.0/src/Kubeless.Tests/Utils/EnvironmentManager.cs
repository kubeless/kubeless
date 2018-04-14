﻿using System;
using System.Collections.Generic;
using System.IO;
using System.Text;

namespace Kubeless.Tests.Utils
{
    public static class EnvironmentManager
    {

        public static FunctionEnvironment CreateEnvironment(string basePath, string functionFileName)
        {
            var environmentPath = Path.Combine(basePath, functionFileName, Guid.NewGuid().ToString());

            EnsureDirectoryIsClear(environmentPath);

            var functionFiles = Directory.EnumerateFiles(basePath, $"{functionFileName}.*");

            CopyFunctionsFiles(functionFiles, environmentPath);

            var environment = new FunctionEnvironment(environmentPath, functionFileName);

            Environment.SetEnvironmentVariable("DOTNETCORE_HOME", environment.PackagesPath);
            Environment.SetEnvironmentVariable("DOTNETCORESHAREDREF_VERSION", "2.0.6"); //TODO: Get Higher available version on computer

            return environment;
        }

        private static void EnsureDirectoryIsClear(string directory)
        {
            if (Directory.Exists(directory))
                Directory.Delete(directory, recursive: true);
            Directory.CreateDirectory(directory);
        }

        public static void CopyFunctionsFiles(IEnumerable<string> files, string destination)
        {
            foreach (var f in files)
            {
                var name = Path.GetFileName(f);
                File.Copy(f, Path.Combine(destination, name));
            }
        }
    }
}
