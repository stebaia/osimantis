import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../../core/di/injection.dart';
import '../../../../core/theme/app_colors.dart';
import '../bloc/chat_bloc.dart';
import '../widgets/message_bubble.dart';
import '../widgets/voice_blob.dart';

/// Home chat-first. Quando non c'è conversazione mostra il blob centrale e il
/// testo dettato (stile schermate di riferimento); appena parte uno scambio
/// diventa una chat con bolle. In basso: campo testo + pulsante microfono.
class ChatPage extends StatelessWidget {
  const ChatPage({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocProvider(
      create: (_) => sl<ChatBloc>(),
      child: const _ChatView(),
    );
  }
}

class _ChatView extends StatefulWidget {
  const _ChatView();

  @override
  State<_ChatView> createState() => _ChatViewState();
}

class _ChatViewState extends State<_ChatView> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: _circleButton(Icons.menu, () {}),
        actions: [_circleButton(Icons.add, () {})],
      ),
      body: SafeArea(
        child: BlocConsumer<ChatBloc, ChatState>(
          listenWhen: (p, c) => p.errorMessage != c.errorMessage && c.errorMessage != null,
          listener: (context, state) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(state.errorMessage!)),
            );
          },
          builder: (context, state) {
            final hasConversation = state.messages.isNotEmpty;
            return Column(
              children: [
                Expanded(
                  child: hasConversation
                      ? _MessageList(messages: state.messages)
                      : _EmptyHero(state: state),
                ),
                _Composer(
                  controller: _controller,
                  state: state,
                  onSend: () {
                    final text = _controller.text;
                    if (text.trim().isEmpty) return;
                    context.read<ChatBloc>().add(ChatMessageSent(text));
                    _controller.clear();
                  },
                  onMic: () {
                    context.read<ChatBloc>().add(
                      ChatListeningToggled(listening: !state.isListening),
                    );
                  },
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _circleButton(IconData icon, VoidCallback onTap) {
    return Padding(
      padding: const EdgeInsets.all(8),
      child: Material(
        color: AppColors.surface,
        shape: const CircleBorder(),
        child: InkWell(
          customBorder: const CircleBorder(),
          onTap: onTap,
          child: SizedBox(
            width: 44,
            height: 44,
            child: Icon(icon, color: AppColors.textPrimary, size: 22),
          ),
        ),
      ),
    );
  }
}

/// Vista "vuota": grande domanda/trascrizione + blob, come nelle immagini.
class _EmptyHero extends StatelessWidget {
  const _EmptyHero({required this.state});
  final ChatState state;

  @override
  Widget build(BuildContext context) {
    final showTranscript = state.transcript.isNotEmpty;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Spacer(),
          Text(
            showTranscript ? state.transcript : 'Raccontami qualcosa\nsulle tue relazioni',
            textAlign: TextAlign.center,
            style: const TextStyle(
              fontSize: 26,
              height: 1.3,
              color: AppColors.textPrimary,
            ),
          ),
          const Spacer(),
          VoiceBlob(active: state.isListening || state.isSending),
          const SizedBox(height: 16),
          Text(
            state.isListening ? 'Ascolto...' : 'Tocca il microfono per parlare',
            style: const TextStyle(color: AppColors.textSecondary),
          ),
          const SizedBox(height: 24),
        ],
      ),
    );
  }
}

class _MessageList extends StatelessWidget {
  const _MessageList({required this.messages});
  final List messages;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      itemCount: messages.length,
      itemBuilder: (context, i) => MessageBubble(message: messages[i]),
    );
  }
}

/// Barra in basso: campo testo + microfono.
class _Composer extends StatelessWidget {
  const _Composer({
    required this.controller,
    required this.state,
    required this.onSend,
    required this.onMic,
  });

  final TextEditingController controller;
  final ChatState state;
  final VoidCallback onSend;
  final VoidCallback onMic;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: controller,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => onSend(),
              decoration: InputDecoration(
                hintText: 'Scrivi un messaggio...',
                filled: true,
                fillColor: AppColors.surface,
                contentPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(24),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          _RoundAction(
            icon: state.isListening ? Icons.stop : Icons.mic,
            highlighted: state.isListening,
            onTap: onMic,
          ),
          const SizedBox(width: 8),
          _RoundAction(
            icon: Icons.arrow_upward,
            highlighted: true,
            onTap: state.isSending ? null : onSend,
          ),
        ],
      ),
    );
  }
}

class _RoundAction extends StatelessWidget {
  const _RoundAction({required this.icon, required this.onTap, this.highlighted = false});
  final IconData icon;
  final VoidCallback? onTap;
  final bool highlighted;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: highlighted ? AppColors.primary : AppColors.surface,
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: SizedBox(
          width: 48,
          height: 48,
          child: Icon(icon, color: highlighted ? Colors.white : AppColors.textPrimary),
        ),
      ),
    );
  }
}
