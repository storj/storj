// Package mobile contains the simplified mobile APIs to Storj Network.
//
// For API limitations see https://github.com/ethereum/go-ethereum/blob/461291882edce0ac4a28f64c4e8725b7f57cbeae/mobile/doc.go#L23
//
//
// build loop for development
//    watchrun gobind -lang=java -outdir=../mobile-out storj.io/storj/mobile == pt skipped ../mobile-out
//
// gomobile bind -target android
//
// To use:
//    gomobile bind -target android
//
// Create a new project in AndroidStudio
//
// Copy mobile-source.jar and mobile.aar into `AndroidStudioProjects\MyApplication\app\libs\`
//
// Modify build.gradle to also find *.aar files:
//   implementation fileTree(dir: 'libs', include: ['*.jar', '*.aar'])
//
// See example Java file
package mobile
