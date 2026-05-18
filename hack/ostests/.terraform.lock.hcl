# This file is maintained automatically by "tofu init".
# Manual edits may be lost in future updates.

provider "registry.opentofu.org/hashicorp/aws" {
<<<<<<< HEAD
  version     = "6.44.0"
  constraints = "~> 6.0"
  hashes = [
    "h1:AdWIZaj+0tEkvbXtmIVdqx5q8mVakrBpG0CucnO+PLY=",
    "h1:Dsk4M5n5pukSt3RHO2oxGJkR+GaDwmU5XQgwTTW/NvI=",
    "h1:J2ATevayOqbbLvUSQGkBfRYh3/bsgT++lbXHefsph7U=",
    "h1:LE/0hoppcdAjbnmMZ5VvqirkJD/VXFWI7lhyTcwoCe8=",
    "h1:OEUfvDxFYeBuqgF7ijlSwGVYnM0uRAWUqk4pX1h4Qw8=",
    "h1:PP8Kpm5Uk75TcnL3wanjq+ac1sX1jKCWNO7MXOEnJQ4=",
    "h1:RYbPW3XdjsdstK6G5Qaf5J09/vzc5b/nFD10s4/cMOQ=",
    "h1:WZUA+j08G7V+4F83+/4oMlCnr/vz1ctqwgTJC/gFss4=",
    "h1:dh8Ix6kSe+XlbkkijTTwk8h2HKRUxzuQxZTANZOTF7g=",
    "h1:hRv170kpszsz7xDJBA9Wc6moVjPJz6FhXXYA7Ir1jZE=",
    "h1:nnkrhJv02OdUdVsoKca2veqHX06nCnheOJOx57YvL0s=",
    "h1:o7isF0sNduBp5aQhSKHroYb8/tvxsxvilW7CnKbh9CU=",
    "h1:qNzwY5ZfurtM1GDMnqCkqG3TyiwEakmCoM+72xNY2cQ=",
    "h1:uUZwHH3dLKbMyC13jgim8/kdBg15K1vR9WPMvj1fw0Q=",
    "h1:xwuzEECrsUaJZ5HDdsrgrbvE9asGc+jYYbnblB97rwA=",
    "zh:2a61db8c7ab08a4df5302cbd36b6a5e4915d29270d2d0ade4baf2bd541d0ca04",
    "zh:3b6fc9bcdec7c1ae2acfce7a258c32196f3218e9ed4089237b02ec31d7c97964",
    "zh:3d50690ee83c7203b3aea6319c3b1c131354bd2ee027e56dc94b547e346f9c26",
    "zh:46a49494db3589b6941f4405b7508d02381e6666d75c600f8b5656816aea9c9b",
    "zh:51d334a594e167001b8ad4842d650495fe0118bcff7371c85256a10ac413dee3",
    "zh:591fcf5e50ce0ef8caa666cc1534bb3e338ff71279e8ad162aa2a81c87598618",
    "zh:5f470c6880033f27aff77fa1d9c54ab32c7daae3f617d10fed393325f317bdea",
    "zh:8fec40a1433179b51a533adf473c494d8dd580b994f11c3a1590f85554c61391",
    "zh:bcd6c7990a946f70ecb009b3516f32ab718827f019a8822b6b464df04e31e8b7",
    "zh:be940702e9f97b75e3830e2453a219a8e4379b85a4e8881b4277ec28c4b0f99d",
    "zh:c212e2628c98a69cd83e5e4a09cce34c984e5130045fe64848af6aad12201bbb",
    "zh:d26e946c0835d8034708843a35471625c2dc54a359a6f25290150daf7daf3b30",
    "zh:d3965e6b0202204ef4d80d58bb870abb1c944025cac3e6b423bc4492d6cd9365",
    "zh:e2a71e1d0ec03f920a5a962871d9270af3a066ff0c0c55ee48f97c544fbfe93a",
    "zh:f472d8603f14d6e34627d00d3368f1304821d4f2ce4334a155ff75877a1b5364",
=======
  version     = "6.43.0"
  constraints = "~> 6.0"
  hashes = [
    "h1:+KqeS3Q6D1NhSzLZmRXoHL1Kzvf30oyALRMvUBMZ8Nc=",
    "h1:3D9yhPDlUFWY4abKnRKJ0VdqpQA8IY5M1tsVtTI8+wc=",
    "h1:4YZrG9IunRIrkcAgE6qV42qguuq3fCui84BNHouGMKs=",
    "h1:4qsQKGEOXVaMUOyqCjUcC4+30KmJJtah9keqB+D+kp0=",
    "h1:54qmZjnQTVQN06YH3yQNDSmh00+roFRfj8XSFep7UYI=",
    "h1:HdCiG7D+bN5YNBcfKGo02fff5jKMfLywEs1+NmccNEs=",
    "h1:J3yxQV4sjTHOIsCzoJwEOO6ZQI2nNyQ7K7R6C8FWBy0=",
    "h1:P4mAJhld0+iKMhiLK54oK3tdRhhuVeWWBq5+zKwrz8E=",
    "h1:PxlQKivBbRceV0svN2OL/p7sZQEacMjUOftw4bHK2ho=",
    "h1:SNrr0bGqSp8layADAv0tdb73DB9b6gtDfnx2ktu9mm4=",
    "h1:WRONI5OW8FuSWm2YXR8K4I6JtBvuJG9dGokcNAYRkUw=",
    "h1:ejQVQfnYGbX7ozHalfy4l44VontY+aw8q63eUqj73EY=",
    "h1:kCnEcXYwJCJd/PohXF9DK7BQYfFmMYJh+M6iMy4ScXQ=",
    "h1:kOJ3LBH3GpjH/e3BH5l78ToXaeK6WuWQfTWLL971Rac=",
    "h1:nRNOnEEuX92jMA+XeoQQ/6PoOjzajn0XJOA4kOdiMM4=",
    "zh:108a58036307f9f0687d2583f86936eb75401e43d358ea62f9702acf5f9d70b2",
    "zh:373cb307d87cab9806f8cd71388d7ba451d7bb310a9e2807c7afb80cee6841a2",
    "zh:42a59943c3f46e3e17ab707df0fcf6313916389383866d3017a467ad37993924",
    "zh:5c3b005efb2ec73833cbb80cd9d1bd2fae27586c510f4c58a7ed28d6c4f1df8f",
    "zh:612ee0001c46dae19255252c3a9344bc0fcdda42568508dda558079ed8cf437f",
    "zh:625d1b0c8e4dc0bb43583c4ad0e7a30c030e3e67d919b6c5e8247707b7ca1e6b",
    "zh:92ecce00bdf17b63fcf31fb8eb7679cfbec0c741097554055f810ee879a96094",
    "zh:b1fa126cb4f52c1fc00beda5c3fa34fac41a95f5a2ad3a14751e335bc93ee7ab",
    "zh:b54a80475ac532f44f7c8cb900c48c01780b50f5dae0fe852175f3331ca9371b",
    "zh:bb77f0c00778d5ac3257ded46c1034df656bdd67f3cf29e74e68da376fea1ec3",
    "zh:bedcc72914998315e8faac3997b5eb6ac49269a14e557efd5916c1946e734179",
    "zh:cbf429abd34c247a57ef1b703d059fea30ecf5463ee0d20a2850bd25adb22d5a",
    "zh:cfbc5decdfce32cde83de9f7e10e30b30297246611c69a2243d88a6421d2dd7a",
    "zh:d656ba4ddd17f108397d7dfd28670a7d70a9c4d46229621748e8755783e391fc",
    "zh:f304352bb8936b9d7dc9f57273c50db72768004ed4789b5712e20b96c3342478",
>>>>>>> a3d85fa9 (feat: delegate konnectivity server counting to lease controller)
  ]
}

provider "registry.opentofu.org/hashicorp/external" {
  version     = "2.3.5"
  constraints = "~> 2.0"
  hashes = [
    "h1:+Bk1x5spvWdtMtnaA8JHt+C1q47tJclICAzr386CvNM=",
    "h1:+OsaKKx2awgjh6j/2B3VBP6q4Dqg2Fc0uDUZll66/Hg=",
    "h1:3VK0DggxYvOG1o+2o+araLMi3JQ/wJsJku0+ytaqtLA=",
    "h1:E6h6CZi8stoNsqPhPTamIRLOoyau34fk908BgNkx3mk=",
    "h1:P7l7h0zcT9phXnaUKePMnuc/BIXniWWMuQGdwT5JloQ=",
    "h1:VsIY+hWGvWHaGvGTSKZslY13lPeAtSTxfZRPbpLMMhs=",
    "h1:jcVmeuuz74tdRt2kj0MpUG9AORdlAlRRQ3k61y0r5Vc=",
    "h1:u2Btw1S8eUlI8iYBYorUVJvR9PTkQuEDSQslv5sWAnY=",
    "h1:uSxBhY6B5Ex8dq8Jc+qPgLj+Ishu5+y755d/MsFlAE4=",
    "h1:w0+DlptrKfWlwVwViinb+TlvaqPfDbgHfjWoXElXsdQ=",
    "zh:1fb9aca1f068374a09d438dba84c9d8ba5915d24934a72b6ef66ef6818329151",
    "zh:3eab30e4fcc76369deffb185b4d225999fc82d2eaaa6484d3b3164a4ed0f7c49",
    "zh:4f8b7a4832a68080f0bf4f155b56a691832d8a91ce8096dac0f13a90081abc50",
    "zh:5ff1935612db62e48e4fe6cfb83dfac401b506a5b7b38342217616fbcab70ce0",
    "zh:993192234d327ec86726041eb6d1efb001e41f32e4518ad8b9b162130b65ee9a",
    "zh:ce445e68282a2c4b2d1f994a2730406df4ea47914c0932fb4a7eb040a7ec7061",
    "zh:e305e17216840c54194141fb852839c2cedd6b41abd70cf8d606d6e88ed40e64",
    "zh:edba65fb241d663c09aa2cbf75026c840e963d5195f27000f216829e49811437",
    "zh:f306cc6f6ec9beaf75bdcefaadb7b77af320b1f9b56d8f50df5ebd2189a93148",
    "zh:fb2ff9e1f86796fda87e1f122d40568912a904da51d477461b850d81a0105f3d",
  ]
}

provider "registry.opentofu.org/hashicorp/http" {
  version     = "3.5.0"
  constraints = "~> 3.0"
  hashes = [
    "h1:eClUBisXme48lqiUl3U2+H2a2mzDawS9biqfkd9synw=",
    "h1:yvwvVZ0vdbsTUMru+7Cr0On1FVgDJHAaC6TNvy/OWzM=",
    "zh:0a2b33494eec6a91a183629cf217e073be063624c5d3f70870456ddb478308e9",
    "zh:180f40124fa01b98b3d2f79128646b151818e09d6a1a9ca08e0b032a0b1e9cb1",
    "zh:3e29e1de149dc10bf78620526c7cb8c62cd76087f5630dfaba0e93cda1f3aa7b",
    "zh:4420950200cf86042ec940d0e2c9b7c89966bf556bf8038ba36217eae663bca5",
    "zh:5d1f7d02109b2e2dca7ec626e5563ee765583792d0fd64081286f16f9433bd0d",
    "zh:8500b138d338b1994c4206aa577b5c44e1d7260825babcf43245a7075bfa52a5",
    "zh:b42165a6c4cfb22825938272d12b676e4a6946ac4e750f85df870c947685df2d",
    "zh:b919bf3ee8e3b01051a0da3433b443a925e272893d3724ee8fc0f666ec7012c9",
    "zh:d13b81ea6755cae785b3e11634936cdff2dc1ec009dc9610d8e3c7eb32f42e69",
    "zh:f1c9d2eb1a6b618ae77ad86649679241bd8d6aacec06d0a68d86f748687f4eb3",
  ]
}

provider "registry.opentofu.org/hashicorp/random" {
  version     = "3.8.1"
  constraints = "~> 3.0"
  hashes = [
    "h1:4XzXwVRt9H0n4AZt0fpNc0AuBIsZH2MF6enFO0Tgm/E=",
    "h1:EHn3jsqOKhWjbg0X+psk0Ww96yz3N7ASqEKKuFvDFwo=",
    "h1:HTRqTUdrtlIPsvneQRTTGUfU46vHJ0uI7lf26SNVSoo=",
    "h1:K/OIbLGX0YNiuoDXlpkerSWyv+bjS97Z6YGUCGePPAw=",
    "h1:LsYuJLZcYl1RiH7Hd3w90Ra5+k5cNqfdRUQXItkTI8Y=",
    "h1:ZmWv79WB2oB8IuUyTylbl9zEeWyJVvjd0Uac+Lk6LVw=",
    "h1:bXd92D1e8ahgUbZNQPoOe+VK+WfNRMP8wU3meagUtB4=",
    "h1:nz02g0rqNAJ+KO1Lq5Krfp3qJCuhA4bJuV8jtUw99ss=",
    "h1:tZP70yQDl6mKnTsDtx6tS6GReVb1lgb7WnlIIneXsHY=",
    "zh:25c458c7c676f15705e872202dad7dcd0982e4a48e7ea1800afa5fc64e77f4c8",
    "zh:2edeaf6f1b20435b2f81855ad98a2e70956d473be9e52a5fdf57ccd0098ba476",
    "zh:44becb9d5f75d55e36dfed0c5beabaf4c92e0a2bc61a3814d698271c646d48e7",
    "zh:7699032612c3b16cc69928add8973de47b10ce81b1141f30644a0e8a895b5cd3",
    "zh:86d07aa98d17703de9fbf402c89590dc1e01dbe5671dd6bc5e487eb8fe87eee0",
    "zh:8c411c77b8390a49a8a1bc9f176529e6b32369dd33a723606c8533e5ca4d68c1",
    "zh:a5ecc8255a612652a56b28149994985e2c4dc046e5d34d416d47fa7767f5c28f",
    "zh:aea3fe1a5669b932eda9c5c72e5f327db8da707fe514aaca0d0ef60cb24892f9",
    "zh:f56e26e6977f755d7ae56fa6320af96ecf4bb09580d47cb481efbf27f1c5afff",
  ]
}

provider "registry.opentofu.org/hashicorp/tls" {
  version     = "4.2.1"
  constraints = "~> 4.0"
  hashes = [
    "h1:ALyauaoiauOEgiDjFt8gFHyvzs8JLBcHAEtIqDsE3Rg=",
    "h1:ApZvTsHD9LJxRJiTlZnXCRTGQ92YUCDXUxNwaVbsQyQ=",
    "h1:LFoLeANH42K6k4aH9zW+xN8pC0ls2flsbGeQTdEnABM=",
    "h1:RZDD1y8qrxf7+gdnfFcGP0G6GDe/kv4zNDNdg1HpuSQ=",
    "h1:Vr77TEfxpe/RLNWDDGI1Wz3rDFN1M3dLNMFSk4DKAAo=",
    "h1:ZT6bZoEZh729x6ax/Xe5eEcBbXZ5HGv2i7ijIh5k74I=",
    "h1:ZilRQg3gaNxvWpwnrjV3ZyU4dXI0yQfgsxu2swX9E14=",
    "h1:p+7dQZVz/XpgVLmH00WLFAxDiSWlo/+LXFft0qM4aLg=",
    "h1:vjihqyZJK3CyXKYpKn4KjqeUBiLCDi26DNgVpzfMm+Y=",
    "zh:0435b85c1aa6ac9892e88d99eaae0b1712764b236bf469c114c6ff4377b113d6",
    "zh:3413d6c61a6a1db2466200832e1d86b2992b81866683b1b946e7e25d99e8daf9",
    "zh:4e7610d4c05fee00994b851bc5ade704ae103d56f28b84dedae7ccab2148cc3f",
    "zh:5d7d29342992c202f748ff72dcaa1fa483d692855e57b87b743162eaf12b729a",
    "zh:7db84d143330fcc1f6f2e79b9c7cc74fdb4ddfe78d10318d060723d6affb8a5c",
    "zh:b7fb825fd0eccf0ea9afb46627d2ec217d2d99a5532de8bcbdfaa0992d0248e2",
    "zh:cb8ca2de5f7367d987a23f88c76d80480bcc49da8bdc3fd24dd9b19d3428d72d",
    "zh:eb88588123dd53175463856d4e2323fb0da44bdcf710ec34f2cad6737475638b",
    "zh:f92baceb82d3a1e5b6a34a29c605b54cae8c6b09ea1fffb0af4d036337036a8f",
  ]
}

provider "registry.opentofu.org/opentffoundation/local" {
  version     = "2.7.0"
  constraints = "~> 2.0"
  hashes = [
    "h1:1A5Z42Wlqhd28AMOYuYjo3WtXx8bbfHOaD0Z1SXqlIw=",
    "h1:5UEp9H2W/lSljz4TEojByD54nlvxTtwp8GPcGshkl1s=",
    "h1:RfLBbtfwURWC1AOOLttbNTYvZQ76wqmrfQJRkxU+lmk=",
    "h1:UHBJIZrB2o6QNymPs6pTiLfRxqQ5UgdTFQ6IwxQvjLo=",
    "h1:bKZhLyajpK7pEizfT3rOpfhUl+gLSQviaX7cYat3WB0=",
    "h1:bQF+RJ94SynogfC53W5lipTR/raOBRNoc+WVxHvAizg=",
    "h1:d8i24AmMP2K7N7H7OuQirDW6IhVbrjWI2AUfWHN2eek=",
    "h1:y4Z2q/vw3DUN+0KOunHUsptXPnO/7rDs9zSLdIdtwMk=",
    "h1:ztAhzyJDidK0Lo0MXxbjlI/0Zkgw4oQvlfZ0ggHi8p4=",
    "zh:39e037a963356e583d90d509d82f6dc19914ef5c66970fb166db414f035468f4",
    "zh:5292e51488d40d6c2b365daa9a406144c3fa3f769f1c03065adb4757d41c6ea0",
    "zh:62db48adf8676e8c67f923352a4acb8e52470220ecaa0c9e21a660f359fd5446",
    "zh:6d5f4555371edde0975b5c2ce5fb048be737ea5dc9aab75c8f9fe37f37bf7850",
    "zh:790ab029516ee126a2b5a122ab0638c09585c71c109b91cefc794a4ecc2ba32e",
    "zh:7b7410b923c17a3495e416b940dbef7ee6e2e82298ea2f5b7f9a0e4c2cad4b69",
    "zh:8baa1caf36ba2b0b63e91cd00750e643d21f13535dce04ae824a1211537c6867",
    "zh:aebc221a0da83e970c737c71e76701df731c6f8d70e56ead85bc1f83996f852d",
    "zh:b3c3ee356591800b11d45fb0bb7d39c8eb3a2141c56dd87808b1fcdc9380816c",
  ]
}
