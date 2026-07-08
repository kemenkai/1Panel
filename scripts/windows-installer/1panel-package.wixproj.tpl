<Project Sdk="WixToolset.Sdk/7.0.0">
  <PropertyGroup>
    <OutputType>Package</OutputType>
    <InstallerPlatform>x64</InstallerPlatform>
    <SuppressValidation>true</SuppressValidation>
    <AcceptEula>wix7</AcceptEula>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="WixToolset.UI.wixext" Version="7.0.0" />
    <PackageReference Include="WixToolset.Util.wixext" Version="7.0.0" />
  </ItemGroup>
</Project>
