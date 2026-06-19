import 'package:speech_to_text/speech_to_text.dart';

/// Wrapper sottile su speech_to_text: isola il plugin dal resto dell'app, così
/// il bloc dipende da un'interfaccia semplice (init/start/stop) ed è sostituibile
/// nei test.
class SpeechService {
  final SpeechToText _speech = SpeechToText();
  bool _available = false;

  /// Inizializza il riconoscimento vocale (chiede i permessi al sistema).
  /// Restituisce true se disponibile.
  Future<bool> init() async {
    _available = await _speech.initialize();
    return _available;
  }

  bool get isAvailable => _available;

  /// Avvia l'ascolto. [onResult] riceve la trascrizione parziale/finale in IT.
  Future<void> start(void Function(String transcript) onResult) async {
    if (!_available) return;
    await _speech.listen(
      onResult: (r) => onResult(r.recognizedWords),
      listenOptions: SpeechListenOptions(localeId: 'it_IT', partialResults: true),
    );
  }

  Future<void> stop() => _speech.stop();

  bool get isListening => _speech.isListening;
}
